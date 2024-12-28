package snail_batcher

import (
	"fmt"
	"log/slog"
	"time"
)

type SnailBatcher[T any] struct {
	batcSize   int
	batch      []T
	inputChan  chan T
	closeChan  chan struct{}
	windowSize time.Duration
	nextWindow time.Time
	outputFunc func([]T) error
}

func NewSnailBatcher[T any](
	windowSize time.Duration,
	batcSize int,
	outputFunc func([]T) error,
) *SnailBatcher[T] {
	res := &SnailBatcher[T]{
		batcSize:   batcSize,
		batch:      make([]T, 0, batcSize),
		inputChan:  make(chan T, batcSize),
		closeChan:  make(chan struct{}),
		windowSize: windowSize,
		nextWindow: time.Now().Add(windowSize),
		outputFunc: outputFunc,
	}

	go res.workerLoop()

	return res
}

func (sb *SnailBatcher[T]) InputChan() chan T {
	return sb.inputChan
}

func (sb *SnailBatcher[T]) Add(item T) {
	sb.inputChan <- item
}

func (sb *SnailBatcher[T]) Close() {
	close(sb.closeChan)
}

func (sb *SnailBatcher[T]) workerLoop() {
	for {
		select {
		case _, open := <-sb.closeChan:
			if !open {
				slog.Debug("snail batcher closed")
				return
			}
		case item := <-sb.inputChan:
			sb.batch = append(sb.batch, item)
			if len(sb.batch) >= sb.batcSize {
				sb.flush()
			}
		case <-time.After(sb.nextWindow.Sub(time.Now())):
			sb.flush()
		}
	}
}

func (sb *SnailBatcher[T]) flush() {
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
