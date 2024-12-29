package snail_batcher

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

type SnailBatcher[T any] struct {
	batchSize       int
	threadSafeFlush bool
	batch           []T
	HasPendingTick  atomic.Bool
	inputChan       chan queueItem[T]
	windowSize      time.Duration
	outputFunc      func([]T) error
}

// Turns out it is faster just having everything in one queue/channel
type queueItemType int

const (
	queueItemAdd queueItemType = iota
	queueItemClose
	queueItemManualFlush
	queueItemTicFlush
)

type queueItem[T any] struct {
	Item T
	Type queueItemType
}

func NewSnailBatcher[T any](
	windowSize time.Duration,
	batchSize int,
	queueSize int,
	threadSafeFlush bool, // false, means we will not copy the batch slice, but re-use an internal buffer
	outputFunc func([]T) error,
) *SnailBatcher[T] {
	res := &SnailBatcher[T]{
		batchSize:       batchSize,
		threadSafeFlush: threadSafeFlush,
		batch:           make([]T, 0, batchSize),
		inputChan:       make(chan queueItem[T], queueSize),
		windowSize:      windowSize,
		outputFunc:      outputFunc,
	}

	go res.workerLoop()

	return res
}

func (sb *SnailBatcher[T]) Add(item T) {
	sb.inputChan <- queueItem[T]{Item: item, Type: queueItemAdd}
}

func (sb *SnailBatcher[T]) AddMany(items []T) {
	for _, item := range items {
		sb.Add(item)
	}
}

func (sb *SnailBatcher[T]) Flush() {
	sb.inputChan <- queueItem[T]{Type: queueItemManualFlush}
}

func (sb *SnailBatcher[T]) Close() {
	sb.inputChan <- queueItem[T]{Type: queueItemClose}
}

func (sb *SnailBatcher[T]) workerLoop() {

	ticker := time.NewTicker(sb.windowSize)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if sb.HasPendingTick.CompareAndSwap(false, true) {
				sb.inputChan <- queueItem[T]{Type: queueItemTicFlush}
			}
		}
	}()

	// Turns out selecting from multiple channels is REALLY expensive,
	// so we just use one channel for everything
	for item := range sb.inputChan {
		switch item.Type {
		case queueItemAdd:
			sb.batch = append(sb.batch, item.Item)
			if len(sb.batch) >= sb.batchSize {
				sb.flush(false, true)
			}
		case queueItemManualFlush:
			sb.flush(false, true)
		case queueItemTicFlush:
			sb.flush(true, true)
		case queueItemClose:
			slog.Debug("flushing and closing batcher")
			sb.flush(false, false)
			return
		}
	}
}

func (sb *SnailBatcher[T]) flush(isTick bool, retryAllowed bool) {
	if isTick {
		defer sb.HasPendingTick.Store(false)
	}

	if len(sb.batch) == 0 {
		return
	}

tryAgain:

	var err error
	if sb.threadSafeFlush {
		err = sb.outputFunc(copyArray(sb.batch))
	} else {
		err = sb.outputFunc(sb.batch)
	}
	if err != nil {
		slog.Error(fmt.Sprintf("error when flushing batch: %v", err))
		if retryAllowed {
			time.Sleep(1 * time.Second)
			goto tryAgain
		}
	}

	sb.batch = sb.batch[:0]
}

func copyArray[T any](arr []T) []T {
	res := make([]T, len(arr))
	copy(res, arr)
	return res
}
