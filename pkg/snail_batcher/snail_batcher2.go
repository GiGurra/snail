package snail_batcher

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

/**
This used to be implemented using channels. But it turns out having multiple goroutines write to the same channel is really slow.
10 routines halves the performance. 100 routines and you are at 25% of the performance. at 1000 routines you are at 10%. and so on.
Plain old mutexes seem faster :S. The idea now is to use a mutex to fill a slice, then when it reaches the batch size,
we copy it to a new send it to an output channel. This way we offload the processing to a separate goroutine, and can
continue to fill the slice.

The new solution is about 2.5x faster for 1 go-routine, and 6x faster for 10_000 go-routines.
*/

type SnailBatcher2[T any] struct {
	batchSize int
	queueSize int
	batchChan chan []T
	queue     []T

	//mutex lock
	lock sync.Mutex

	windowSize time.Duration
	outputFunc func([]T) error
}

func NewSnailBatcher2[T any](
	windowSize time.Duration,
	batchSize int,
	queueSize int,
	outputFunc func([]T) error,
) *SnailBatcher2[T] {
	res := &SnailBatcher2[T]{
		batchSize:  batchSize,
		queueSize:  queueSize,
		batchChan:  make(chan []T, max(2, queueSize/batchSize)), // some reasonable back pressure
		queue:      make([]T, 0, queueSize),                     // TODO: Improve the perf with circular buffer? Or slice pool?
		windowSize: windowSize,
		outputFunc: outputFunc,
	}

	go res.workerLoop()

	return res
}

func (sb *SnailBatcher2[T]) Add(item T) {
	sb.lock.Lock()
	defer sb.lock.Unlock()
	sb.addUnsafe(item)
}

func (sb *SnailBatcher2[T]) addUnsafe(item T) {
	sb.queue = append(sb.queue, item)
	if len(sb.queue) >= sb.queueSize {
		sb.flushUnsafe()
	}
}

func (sb *SnailBatcher2[T]) AddMany(items []T) {
	sb.lock.Lock()
	defer sb.lock.Unlock()
	for _, item := range items {
		sb.addUnsafe(item)
	}
}

func (sb *SnailBatcher2[T]) Flush() {
	sb.lock.Lock()
	defer sb.lock.Unlock()
	sb.flushUnsafe()
}

func (sb *SnailBatcher2[T]) flushUnsafe() {
	if len(sb.queue) == 0 { // never send empty slice, since it's a signal to close the internal worker routine
		return
	}
	cpy := make([]T, len(sb.queue))
	copy(cpy, sb.queue)
	sb.queue = sb.queue[:0]
	sb.batchChan <- cpy
}

func (sb *SnailBatcher2[T]) Close() {
	sb.lock.Lock()
	defer sb.lock.Unlock()
	sb.flushUnsafe()
	sb.batchChan <- []T{} // indicates a close
}

func (sb *SnailBatcher2[T]) workerLoop() {

	ticker := time.NewTicker(sb.windowSize)
	defer ticker.Stop()

	go func() {
		for range ticker.C { // ugly, but it works :)
			if len(sb.batchChan) == 0 {
				sb.Flush()
			}
		}
	}()

	for batch := range sb.batchChan {
		for len(batch) == 0 {
			slog.Debug("batcher received close signal, stopping")
			return
		}

		err := sb.outputFunc(batch)
		if err != nil {
			slog.Error(fmt.Sprintf("error when flushing batch: %v", err))
			// TODO: Forward errors, somehow. Or maybe just log them? Or provide retry policy, idk...
		}
	}
}
