package snail_batcher

import (
	"fmt"
	"log/slog"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// This is an entirely new batcher based on a new algorithm.
// It's basically an n-buffer implementation - defaulting to triple buffering.
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
// Why not just use channels? Because they're too slow!
// Why not just regular mutexes? Same reason!

// TODO: Support error callbacks

type SnailBatcher[T any] struct {
	batchSize int
	queueSize int

	pushChan          chan []T
	pullChan          chan []T
	currentBackBuffer []T

	lock         sync.Mutex
	idiotLock    atomic.Bool
	useIdiotLock bool

	timeout    time.Duration
	outputFunc func([]T) error
}

func NewSnailBatcher[T any](
	batchSize int,
	queueSize int,
	useIdiotLock bool, // improves performance for clients under high throughput, mostly on macos
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

		useIdiotLock: useIdiotLock,

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
	sb.lockMutex()
	defer sb.unlockMutex()
	sb.addUnsafe(item)
}

func (sb *SnailBatcher[T]) addUnsafe(item T) {
	if sb.currentBackBuffer == nil {
		sb.currentBackBuffer = <-sb.pullChan
	}
	sb.currentBackBuffer = append(sb.currentBackBuffer, item)
	if len(sb.currentBackBuffer) >= sb.batchSize {
		sb.flushUnsafe()
	}
}

// lockMutex. When to use idiotLock? In general where there are
// many writes and on macOS, it can give 10x the performance or more.
// It's partially a spinlock, so it's not ideal for many cases
// of low contention - but for some! Generally only use client side
// when you know you have many writers.

func (sb *SnailBatcher[T]) lockMutex() {

	if sb.useIdiotLock {

		for !sb.idiotLock.CompareAndSwap(false, true) {
			if rand.Float32() < 0.001 {
				time.Sleep(1 * time.Microsecond)
			} else {
				runtime.Gosched()
			}
		}

	} else {
		sb.lock.Lock()
	}
}

func (sb *SnailBatcher[T]) unlockMutex() {
	if sb.useIdiotLock {
		sb.idiotLock.Store(false)
	} else {
		sb.lock.Unlock()
	}
}

func (sb *SnailBatcher[byte]) AddMany(newItems []byte) {
	sb.lockMutex()
	defer sb.unlockMutex()

	for itemsAdded, itemsLeftToAdd := 0, len(newItems); itemsLeftToAdd > 0; {

		if sb.currentBackBuffer == nil {
			sb.currentBackBuffer = <-sb.pullChan
		}

		spaceLeftInCurrentBackBuffer := sb.batchSize - len(sb.currentBackBuffer)
		itemsToAddThisBatch := min(spaceLeftInCurrentBackBuffer, itemsLeftToAdd)
		sb.currentBackBuffer = append(sb.currentBackBuffer, newItems[itemsAdded:itemsAdded+itemsToAddThisBatch]...)

		if len(sb.currentBackBuffer) >= sb.batchSize {
			sb.flushUnsafe()
		}

		itemsAdded += itemsToAddThisBatch
		itemsLeftToAdd -= itemsToAddThisBatch
	}
}

func (sb *SnailBatcher[T]) Flush() {
	sb.lockMutex()
	defer sb.unlockMutex()
	sb.flushUnsafe()
}

func (sb *SnailBatcher[T]) flushUnsafe() {
	if len(sb.currentBackBuffer) == 0 { // never send empty slice, since it's a signal to close the internal worker routine
		return
	}
	sb.pushChan <- sb.currentBackBuffer
	sb.currentBackBuffer = nil
}

func (sb *SnailBatcher[T]) Close() {
	sb.lockMutex()
	defer sb.unlockMutex()
	sb.flushUnsafe()
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
