package snail_batcher

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

/**
This used to be implemented using channels. But it turns out having multiple goroutines write to the same channel is really slow.
10 routines halves the performance. 100 routines and you are at 25% of the performance. at 1000 routines you are at 10%. and so on.
Plain old mutexes seem faster :S.
*/

type SnailBatcher2[T any] struct {
	batchSize       int
	threadSafeFlush bool
	queueSize       int
	batch           []T
	queue           []queueItem[T]
	HasPendingTick  atomic.Bool

	//mutex lock
	lock sync.Mutex

	//empty and full locks
	notEmpty *sync.Cond
	notFull  *sync.Cond

	windowSize time.Duration
	outputFunc func([]T) error
}

func NewSnailBatcher2[T any](
	windowSize time.Duration,
	batchSize int,
	queueSize int,
	threadSafeFlush bool, // false, means we will not copy the batch slice, but re-use an internal buffer
	outputFunc func([]T) error,
) *SnailBatcher2[T] {
	res := &SnailBatcher2[T]{
		batchSize:       batchSize,
		threadSafeFlush: threadSafeFlush,
		queueSize:       queueSize,
		batch:           make([]T, 0, batchSize),
		queue:           make([]queueItem[T], 0, queueSize),
		windowSize:      windowSize,
		outputFunc:      outputFunc,
	}

	res.notEmpty = sync.NewCond(&res.lock)
	res.notFull = sync.NewCond(&res.lock)

	go res.workerLoop()

	return res
}

func (sb *SnailBatcher2[T]) Add(item T) {
	sb.notFull.L.Lock()
	defer sb.notFull.L.Unlock()
	sb.addUnsafe(item)
}

func (sb *SnailBatcher2[T]) addUnsafe(item T) {
	for len(sb.queue) >= sb.queueSize {
		sb.notFull.Wait()
	}
	sb.queue = append(sb.queue, queueItem[T]{Item: item, Type: queueItemAdd})
	if len(sb.queue)+len(sb.batch) >= sb.queueSize {
		sb.triggerPollUnsafe()
	}
}

func (sb *SnailBatcher2[T]) triggerPollUnsafe() {
	sb.notEmpty.Signal()
}

func (sb *SnailBatcher2[T]) AddMany(items []T) {
	sb.notFull.L.Lock()
	defer sb.notFull.L.Unlock()
	for _, item := range items {
		sb.addUnsafe(item)
	}
}

func (sb *SnailBatcher2[T]) Flush() {
	sb.notEmpty.L.Lock()
	defer sb.notEmpty.L.Unlock()
	sb.queue = append(sb.queue, queueItem[T]{Type: queueItemManualFlush})
	sb.triggerPollUnsafe()
}

func (sb *SnailBatcher2[T]) Close() {
	sb.notEmpty.L.Lock()
	defer sb.notEmpty.L.Unlock()
	sb.queue = append(sb.queue, queueItem[T]{Type: queueItemClose})
	sb.triggerPollUnsafe()
}

func (sb *SnailBatcher2[T]) workerLoop() {
	// TODO: Re-enable ticker!
	//ticker := time.NewTicker(sb.windowSize)
	//defer ticker.Stop()
	//
	//go func() {
	//	for range ticker.C {
	//		if sb.HasPendingTick.CompareAndSwap(false, true) {
	//			sb.inputChan <- queueItem[T]{Type: queueItemTicFlush}
	//		}
	//	}
	//}()

	sb.notEmpty.L.Lock()
	defer sb.notEmpty.L.Unlock()

	//cmdQue := make([]queueItem[T], 0, sb.queueSize)

	for {
		for len(sb.queue) == 0 {
			sb.notEmpty.Wait()
		}

		//slog.Info(fmt.Sprintf("Processing %d items", len(sb.queue)))

		//cmdQue = append(cmdQue, sb.queue...)
		//sb.queue = sb.queue[:0] // will get us back to waiting for the next signal
		//sb.notFull.Broadcast()
		//sb.notEmpty.L.Unlock()

		// Now we can release the lock and process the items we got,
		// But we must lock it again for the wait to work

		// we have the lock in here
		for _, item := range sb.queue {
			switch item.Type {
			case queueItemAdd:
				sb.batch = append(sb.batch, item.Item)
				if len(sb.batch) >= sb.batchSize {
					sb.flush()
				}
			case queueItemManualFlush:
				sb.flush()
			case queueItemTicFlush:
				sb.flush()
				sb.HasPendingTick.Store(false)
			case queueItemClose:
				slog.Debug("flushing and closing batcher")
				sb.flush()
				return
			}
		}

		sb.queue = sb.queue[:0] // will get us back to waiting for the next signal

		sb.notFull.Broadcast()
		//cmdQue = cmdQue[:0]
		//sb.notEmpty.L.Lock()

	}
}

func (sb *SnailBatcher2[T]) flush() {
	if len(sb.batch) == 0 {
		return
	}

	var err error
	if sb.threadSafeFlush {
		err = sb.outputFunc(copyArray(sb.batch))
	} else {
		err = sb.outputFunc(sb.batch)
	}
	if err != nil {
		slog.Error(fmt.Sprintf("error when flushing batch: %v", err))
		// TODO: Forward errors, somehow. Or maybe just log them? Or provide retry policy, idk...
	}

	sb.batch = sb.batch[:0]
}
