package snail_batcher

import (
	"fmt"
	"log/slog"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

/**
This used to be implemented using channels. But it turns out having multiple goroutines write to the same channel is really slow.
10 routines halves the performance. 100 routines and you are at 25% of the performance. at 1000 routines you are at 10%. and so on.
Plain old mutexes seem faster :S. The idea now is to use a mutex to fill a slice, then when it reaches the batch size,
we copy it to a new send it to an output channel. This way we offload the processing to a separate goroutine, and can
continue to fill the slice.

The new solution with reglar sync mutex is about 2.5x faster for 1 go-routine, and 6x faster for 10_000 go-routines.
BUUUUUT, that's only on linux... When i test it on macos, the regular sync.Mutex is super slow between 2-8 goroutines (slower than channels).

Soooo... I'm trying out a new mutex implementation, idiotMutex, which is close to a spinlock.
It's 10x faster than the channel implementation in all situations, which is good enough - i.e. we always have about 50m/s performance even at 100.000 go-routines.
*/

type SnailBatcher[T any] struct {
	batchSize    int
	queueSize    int
	batchChan    chan []T
	pendingBatch []T

	lock sync.Mutex

	windowSize time.Duration
	outputFunc func([]T) error
}

func NewSnailBatcher[T any](
	windowSize time.Duration,
	batchSize int,
	queueSize int,
	outputFunc func([]T) error,
) *SnailBatcher[T] {
	res := &SnailBatcher[T]{
		batchSize:    batchSize,
		queueSize:    queueSize,
		batchChan:    make(chan []T, max(2, queueSize/batchSize)), // some reasonable back pressure
		pendingBatch: make([]T, 0, batchSize),                     // TODO: Improve the perf with circular buffer? Or slice pool?
		windowSize:   windowSize,
		outputFunc:   outputFunc,
	}

	go res.workerLoop()

	return res
}

func (sb *SnailBatcher[T]) Add(item T) {
	sb.lockMutex()
	defer sb.lock.Unlock()
	sb.addUnsafe(item)
}

func (sb *SnailBatcher[T]) addUnsafe(item T) {
	sb.pendingBatch = append(sb.pendingBatch, item)
	if len(sb.pendingBatch) >= sb.batchSize {
		sb.flushUnsafe()
	}
}

var useIdiotMutex = isMacOS()

func isMacOS() bool {
	return runtime.GOOS == "darwin"
}

func (sb *SnailBatcher[T]) lockMutex() {

	if //goland:noinspection GoBoolExpressions
	useIdiotMutex {

		// macos locks are incredibly slow. The numbers below are just
		// empirical values that seem to work well. :S.
		// Regular locks at low contention are 10x slower than the idiotMutex below.
		for !sb.lock.TryLock() {
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

func (sb *SnailBatcher[T]) AddMany(items []T) {
	sb.lockMutex()
	defer sb.lock.Unlock()
	for _, item := range items {
		sb.addUnsafe(item)
	}
}

func (sb *SnailBatcher[T]) Flush() {
	sb.lockMutex()
	defer sb.lock.Unlock()
	sb.flushUnsafe()
}

func (sb *SnailBatcher[T]) flushUnsafe() {
	if len(sb.pendingBatch) == 0 { // never send empty slice, since it's a signal to close the internal worker routine
		return
	}
	cpy := make([]T, len(sb.pendingBatch))
	copy(cpy, sb.pendingBatch)
	sb.pendingBatch = sb.pendingBatch[:0]
	sb.batchChan <- cpy
}

func (sb *SnailBatcher[T]) Close() {
	sb.lockMutex()
	defer sb.lock.Unlock()
	sb.flushUnsafe()
	sb.batchChan <- []T{} // indicates a close
}

func (sb *SnailBatcher[T]) workerLoop() {

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
