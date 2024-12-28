package snail_batcher

import (
	"log/slog"
	"time"
)

type SnailBatcher[T any] struct {
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
			if len(sb.batch) == cap(sb.batch) {
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

	err := sb.outputFunc(sb.batch)
	if err != nil {
		slog.Error("error in outputFunc: %v, will try again in 1 second", err)
		time.Sleep(1 * time.Second)
		goto tryAgain
	}

	// do something with the inputChan
	sb.nextWindow = time.Now().Add(sb.windowSize)
}
