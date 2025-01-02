package snail_batcher

import (
	"fmt"
	"log/slog"
	"sync"
)

type SnailBatcherMutexBasedPrototype[T any] struct {
	mutex sync.Mutex
	data  []T
}

func NewBufferMutexBasedPrototype[T any](size int) *SnailBatcherMutexBasedPrototype[T] {
	return &SnailBatcherMutexBasedPrototype[T]{
		data: make([]T, 0, size),
	}
}

func (b *SnailBatcherMutexBasedPrototype[T]) PushOne(item T) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.data = append(b.data, item)
	if cap(b.data) == len(b.data) {
		slog.Info(fmt.Sprintf("Flushing buffer. nWritten: %d", len(b.data)))
		b.data = b.data[:0]
	}
}
