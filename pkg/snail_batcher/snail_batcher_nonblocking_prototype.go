package snail_batcher

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync/atomic"
	"time"
)

// This is a prototype of a non-blocking version of the SnailBatcher.
// On linux x86 (7950X3D), it is a little slower than the spinlock version during high contention,
// while during low contention, the non-blocking version is faster.
// However, on MacOS/AppleSilicon the non-blocking version is way too slow,
// and the spinlock version is 5x faster during high contention.
// The same is true for spinlocks themselves vs atomic ops on Apple Silicon, see snail_batcher_impl_perf_compar_test.go.

const CacheLinePadding = 256 // arm: 32, amd: 64, intel 16-256 :S

type preallocedBatch[T any] struct {
	_         [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	writePos  atomic.Uint64
	_         [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	nWritten  atomic.Uint64
	_         [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	data      []T
	finalSize int
}

type SnailBatcherNonBlockingPrototype[T any] struct {
	pushChan     chan *preallocedBatch[T]
	pullChan     chan *preallocedBatch[T]
	currentBatch atomic.Pointer[preallocedBatch[T]]
}

func NewSnailBatcherNonBlockingPrototype[T any](
	batchSize int,
	queueSize int,
	timeout time.Duration,
	outputFunc func([]T) error,
) *SnailBatcherNonBlockingPrototype[T] {

	if queueSize <= 0 {
		panic("queueSize must be greater than 0")
	}

	if queueSize%batchSize != 0 {
		panic("queueSize must be a multiple of batchSize")
	}

	totalBufCount := queueSize/batchSize + 1

	res := &SnailBatcherNonBlockingPrototype[T]{
		pushChan: make(chan *preallocedBatch[T], totalBufCount),
		pullChan: make(chan *preallocedBatch[T], totalBufCount),
	}

	// add totalBufCount to pullChan
	for i := 0; i < totalBufCount; i++ {
		res.pullChan <- &preallocedBatch[T]{data: make([]T, batchSize)}
	}

	res.currentBatch.Store(<-res.pullChan)

	go res.workerLoop(timeout, outputFunc)

	return res
}

func (b *SnailBatcherNonBlockingPrototype[T]) Close() {
	close(b.pushChan)
}

func (b *SnailBatcherNonBlockingPrototype[T]) workerLoop(
	timeout time.Duration,
	outputFunc func([]T) error,
) {

	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			b.Flush()
		}
	}()

	for batch := range b.pushChan {

		// Flush the buffer
		if batch.finalSize != 0 {
			err := outputFunc(batch.data[:batch.finalSize])
			if err != nil {
				slog.Error(fmt.Sprintf("Error flushing buffer: %s", err))
				// TODO: Determine how/if to push errors back to the producer
			}
		}

		// Make the buffer available for reuse
		batch.writePos.Store(0)
		batch.nWritten.Store(0)
		b.pullChan <- batch
	}
}

func (b *SnailBatcherNonBlockingPrototype[T]) Flush() {

	batch := b.currentBatch.Load()

	if batch == nil {
		return // no batch to flush
	}

	// Reset the batch state. Let the flush do its work without interruptions.
	if !b.currentBatch.CompareAndSwap(batch, nil) {
		return // someone else is already flushing this batch
	}

	// Mark the batch's write position as the length of the data.
	// This effectively marks the batch as full so that no more elements can be added to it.
	newWritePos := batch.writePos.Add(uint64(len(batch.data)))

	// Calculate how many real elements are in this batch. This is at maximum the length of the data slice.
	// The atomic counter however may be higher, if other goroutines have attempted to acquire a lot after
	// the batch was marked as full.
	elementsInBatch := min(len(batch.data), int(newWritePos)-len(batch.data))

	// Don't flush empty batches
	if elementsInBatch == 0 {
		// reset the batch state and hand it back
		batch.writePos.Store(0)
		b.currentBatch.Store(batch)
		return
	} else {

		// must wait until all elements are written, i.e. all other goroutines have finished writing
		for batch.nWritten.Load() != uint64(elementsInBatch) {
			//runtime.Gosched()
		}
		batch.finalSize = elementsInBatch

		// Swap the current batch with the new one
		b.pushChan <- batch
		b.currentBatch.Store(<-b.pullChan)
	}
}

func (b *SnailBatcherNonBlockingPrototype[T]) fastFullFlush(batch *preallocedBatch[T]) {

	// Reset the batch state. Let the flush do its work without interruptions.
	if !b.currentBatch.CompareAndSwap(batch, nil) {
		return // someone else is already flushing this batch
	}

	elementsInBatch := len(batch.data)
	// must wait until all elements are written, i.e. all other goroutines have finished writing
	for batch.nWritten.Load() != uint64(elementsInBatch) {
		//runtime.Gosched()
	}
	batch.finalSize = elementsInBatch

	// Swap the current batch with the new one
	b.pushChan <- batch
	b.currentBatch.Store(<-b.pullChan)
}

func (b *SnailBatcherNonBlockingPrototype[T]) Add(item T) {

tryAgain:

	batch := b.currentBatch.Load()
	if batch == nil {
		runtime.Gosched()
		goto tryAgain
	}

	newCount := int(batch.writePos.Add(1))
	if newCount > len(batch.data) {
		// This means someone else is already responsible for flushing this batch
		runtime.Gosched()
		goto tryAgain
	}

	// Add our item to the batch, and increment the number of written elements atomically
	batch.data[newCount-1] = item
	batch.nWritten.Add(1)

	// if the batch is full, and we were the ones to fill it, we are responsible for flushing it
	if newCount == len(batch.data) {
		b.fastFullFlush(batch)
	}
}

func (b *SnailBatcherNonBlockingPrototype[T]) AddMany(items []T) {

	for len(items) != 0 {

		batch := b.currentBatch.Load()
		if batch == nil {
			runtime.Gosched()
			continue
		}

		newCount := int(batch.writePos.Add(uint64(len(items)))) // this is what reserves the slots atomically
		oldCount := min(len(batch.data), newCount-len(items))
		slotsAvailable := max(0, len(batch.data)-oldCount)
		slotsReserved := min(slotsAvailable, len(items))
		if slotsReserved == 0 {
			runtime.Gosched()
			continue
		}

		copy(batch.data[oldCount:], items[:slotsReserved])
		batch.nWritten.Add(uint64(slotsReserved))

		// if the batch is full, flush it
		if newCount >= len(batch.data) {
			b.fastFullFlush(batch)
		}

		items = items[slotsReserved:]
	}
}
