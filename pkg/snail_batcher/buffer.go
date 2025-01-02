package snail_batcher

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync/atomic"
)

const CacheLinePadding = 64 // makes it 2-3x faster :D, due to prevention of false sharing

type Buffer struct {
	_        [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	writePos atomic.Uint64
	_        [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	nWritten atomic.Uint64
	//_        [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	//bufsize  int
	_    [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	data []int64
}

func NewBuffer(size int) *Buffer {
	return &Buffer{
		data: make([]int64, size),
	}
}

func (b *Buffer) PushOne(item int64) {

tryAgain:

	writePos := b.writePos.Add(1) - 1
	if int(writePos) == len(b.data) {
		for b.nWritten.Load() != uint64(len(b.data)) {
			runtime.Gosched() // must wait until all elements are written, i.e. all other goroutines have finished writing
		}
		// we flush
		slog.Info(fmt.Sprintf("Flushing buffer. nWritten: %d", b.nWritten.Load()))
		b.nWritten.Store(0)
		b.writePos.Store(0)
		goto tryAgain
	} else if int(writePos) > len(b.data) {
		runtime.Gosched()
		goto tryAgain
	}

	b.data[writePos] = item
	b.nWritten.Add(1)
}
