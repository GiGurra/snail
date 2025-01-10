package snail_batcher

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"
)

// This is the fundamental building block of snail. It's a system for creating batches of data
// from individual elements. The individual elements are expected to be added by multiple producers
// on different goroutines. The completed batches are then processed by a single worker goroutine.
//
// How it is implemented:
//
// It's basically an n-buffer implementation - recommended to be used with triple buffering.
// It works like this: We have clients pushing data, 2+ buffers, and a worker.
// The worker gets data from clients when buffers have been fully written
// and enqueued to the worker, or a timeout has been reached.
//
// Clients write to an available back buffer. When it's full,
// they push it to the worker over the push channel, and
// then pull a new back buffer from the worker over the pull channel.
//
// Both push and pull channels are buffered.
//
// Each buffer has a max size called batchSize. The number of additional buffers
// available make up the queue. The queue size is given in number of elements, and it
// must be a multiple of the batchSize.
//
// Example: Batch size of 10 and queue size of 10 means we have 2 buffers in total (double buffering).
// Batch size of 10 and queue size of 20 means we have 3 buffers in total (triple buffering).
//
// Why not just use channels? Because they're too slow! see snail_batcher_impl_perf_compar_test.go
// Why not just regular mutexes? Same reason! see snail_batcher_impl_perf_compar_test.go
//
// So what do we do instead? We use a combination of channels and mutexes.
// We have 2 mutexes. One that we spin on (the normal situation), and another we wait on (when we need a new buffer,
// to provide back pressure if the worker is slower than the producers).
// We also have two channels: one for pushing ready batches to the worker, and one for pulling new back buffers
// back from the worker once it is done.
// This approach has been empirically found to have the best performance across Linux and MacOS.
// There is also a pure nonblocking version implemented as a prototype in another file in this package,
// but that one turned out to be way too slow for macos/apple silicon. This is due to apple silicon being
// slow at high contention atomic operations. See snail_batcher_impl_perf_compar_test.go.
//
// Why spin at all, you may ask? Because on mac-os/apple silicon, regular lock waiting is atrociously slow.

// TODO: Support error callbacks

type SnailBatcher[T any] struct {
	batchSize int
	queueSize int

	pushChan chan []T
	pullChan chan []T

	lock              sync.Mutex // We spin on this one as long as there is a buffer available
	currentBackBuffer []T

	newBackBufferLock sync.Mutex // We lock this one when we wait for a buffer to become available

	timeout    time.Duration
	outputFunc func([]T) error
}

func NewSnailBatcher[T any](
	batchSize int,
	queueSize int,
	timeout time.Duration,
	outputFunc func([]T) error,
) *SnailBatcher[T] {

	if batchSize <= 0 {
		panic(fmt.Sprintf("batchSize must be > 0, got %d", batchSize))
	}

	if queueSize <= 0 {
		panic(fmt.Sprintf("queueSize must be > 0, got %d", queueSize))
	}

	if queueSize%batchSize != 0 {
		panic(fmt.Sprintf("queueSize must be a multiple of batchSize, got %d", queueSize))
	}

	totalBufferCount := 1 + queueSize/batchSize

	res := &SnailBatcher[T]{

		batchSize: batchSize,
		queueSize: queueSize,

		pushChan:          make(chan []T, totalBufferCount),
		pullChan:          make(chan []T, totalBufferCount),
		currentBackBuffer: nil,

		timeout:    timeout,
		outputFunc: outputFunc,
	}

	// add buffers to the pull channel
	for i := 0; i < totalBufferCount; i++ {
		res.pullChan <- make([]T, 0, batchSize)
	}

	go res.workerLoop()

	return res
}

func (sb *SnailBatcher[T]) Add(item T) {
	sb.lockSpinLock()
	defer sb.unlockSpinLock()
	sb.addInternal(item)
}

// ensureBackBufferInternal makes sure we have a back buffer available. It is a little complicated.
// The reason is that we want to put all producers/clients to sleep if the worker is overloaded (i.e. back pressure)
func (sb *SnailBatcher[T]) ensureBackBufferInternal() {
	for sb.currentBackBuffer == nil { // this is generally only the case if we have just flushed
		sb.unlockSpinLock()         // we do this to stop others from spinning
		sb.newBackBufferLock.Lock() // here we always want a regular lock, so we don't spin.
		// Whoever grabbed the lock first will now set the new back buffer
		if sb.currentBackBuffer == nil {
			sb.currentBackBuffer = <-sb.pullChan
		}
		sb.newBackBufferLock.Unlock()
		sb.lockSpinLock()
	}
}

func (sb *SnailBatcher[T]) addInternal(item T) {
	sb.ensureBackBufferInternal()
	sb.currentBackBuffer = append(sb.currentBackBuffer, item)
	if len(sb.currentBackBuffer) >= sb.batchSize {
		sb.flushInternal()
	}
}

func (sb *SnailBatcher[T]) lockSpinLock() {
	for !sb.lock.TryLock() {
		runtime.Gosched()
	}
}

func (sb *SnailBatcher[T]) unlockSpinLock() {
	sb.lock.Unlock()
}

func (sb *SnailBatcher[T]) AddMany(newItems []T) {
	sb.lockSpinLock()
	defer sb.unlockSpinLock()

	for len(newItems) != 0 {

		sb.ensureBackBufferInternal()

		availableForWriteInTrg := sb.batchSize - len(sb.currentBackBuffer)
		chunk := newItems
		if len(chunk) > availableForWriteInTrg {
			chunk = chunk[:availableForWriteInTrg]
		}

		sb.currentBackBuffer = append(sb.currentBackBuffer, chunk...)
		if len(sb.currentBackBuffer) >= sb.batchSize {
			sb.flushInternal()
		}

		newItems = newItems[len(chunk):]
	}
}

func (sb *SnailBatcher[T]) Flush() {
	sb.lockSpinLock()
	defer sb.unlockSpinLock()
	sb.flushInternal()
}

func (sb *SnailBatcher[T]) flushInternal() {
	if len(sb.currentBackBuffer) == 0 { // never send empty slice, since it's a signal to close the internal worker routine
		return
	}
	sb.pushChan <- sb.currentBackBuffer
	sb.currentBackBuffer = nil
}

func (sb *SnailBatcher[T]) Close() {
	sb.lockSpinLock()
	defer sb.unlockSpinLock()
	sb.flushInternal()
	sb.pushChan <- []T{} // indicates a close
}

func (sb *SnailBatcher[T]) workerLoop() {

	// Ugly for now, but it works
	ticker := time.NewTicker(sb.timeout)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			sb.Flush()
		}
	}()

	for batch := range sb.pushChan {
		for len(batch) == 0 {
			slog.Debug("batcher received close signal, stopping")
			return
		}

		err := sb.outputFunc(batch)
		if err != nil {
			slog.Error(fmt.Sprintf("error when flushing batch: %v", err))
			// TODO: Forward errors, somehow. Or maybe just log them? Or provide retry policy, idk...
		}

		// zero out the buffer and push it back to the pull channel
		batch = batch[:0]
		sb.pullChan <- batch
	}
}
