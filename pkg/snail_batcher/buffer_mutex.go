package snail_batcher

import (
	"fmt"
	"log/slog"
	"sync"
)

// CacheLinePadding has no effect for the mutex based solution. No clue why :D

type BufferWithMutex struct {
	mutex sync.Mutex
	data  []int64
}

func NewBufferWithMutex(size int) *BufferWithMutex {
	return &BufferWithMutex{
		data: make([]int64, 0, size),
	}
}

func (b *BufferWithMutex) PushOne(item int64) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.data = append(b.data, item)
	if cap(b.data) == len(b.data) {
		slog.Info(fmt.Sprintf("Flushing buffer. nWritten: %d", len(b.data)))
		b.data = b.data[:0]
	}
}
