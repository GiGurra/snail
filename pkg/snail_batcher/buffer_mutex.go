package snail_batcher

import (
	"fmt"
	"log/slog"
	"sync"
)

type BufferWithMutex struct {
	_        [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	mutex    sync.Mutex
	_        [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	writePos int
	//_        [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	//bufsize  int
	_    [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	data []int64
}

func NewBufferWithMutex(size int) *BufferWithMutex {
	return &BufferWithMutex{
		data: make([]int64, size),
	}
}

func (b *BufferWithMutex) PushOne(item int64) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.writePos == len(b.data) {
		// we flush
		slog.Info(fmt.Sprintf("Flushing buffer. nWritten: %d", b.writePos))
		b.data = make([]int64, len(b.data))
		b.writePos = 0
	}

	b.data[b.writePos] = item
	b.writePos++
}
