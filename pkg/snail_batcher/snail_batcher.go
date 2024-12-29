package snail_batcher

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

type SnailBatcher[T any] struct {
	batchSize      int
	batch          []T
	HasPendingTick atomic.Bool
	inputChan      chan queueItem[T]
	windowSize     time.Duration
	nextWindow     time.Time
	outputFunc     func([]T) error
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
	outputFunc func([]T) error,
) *SnailBatcher[T] {
	res := &SnailBatcher[T]{
		batchSize:  batchSize,
		batch:      make([]T, 0, batchSize),
		inputChan:  make(chan queueItem[T], queueSize),
		windowSize: windowSize,
		nextWindow: time.Now().Add(windowSize),
		outputFunc: outputFunc,
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
				sb.flush(false)
			}
		case queueItemManualFlush:
			sb.flush(false)
		case queueItemTicFlush:
			sb.flush(true)
		case queueItemClose:
			sb.flush(false)
			slog.Debug("closing batcher")
			return
		}
	}
}

func (sb *SnailBatcher[T]) flush(isTick bool) {
	if isTick {
		defer sb.HasPendingTick.Store(false)
	}

	if len(sb.batch) == 0 {
		return
	}

tryAgain:

	err := sb.outputFunc(copyArray(sb.batch))
	if err != nil {
		slog.Error(fmt.Sprintf("error when flushing batch: %v", err))
		time.Sleep(1 * time.Second)
		goto tryAgain
	}

	sb.batch = sb.batch[:0]

	// do something with the inputChan
	sb.nextWindow = time.Now().Add(sb.windowSize)
}

func copyArray[T any](arr []T) []T {
	res := make([]T, len(arr))
	copy(res, arr)
	return res
}
