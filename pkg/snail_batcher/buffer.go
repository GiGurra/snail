package snail_batcher

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
)

const CacheLinePadding = 64 // makes it 2-3x faster :D, due to prevention of false sharing

type proccessingBatch[T any] struct {
	_        [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	writePos atomic.Uint64
	_        [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	nWritten atomic.Uint64
	_        [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	data     []T
}

type Buffer[T any] struct {
	_             [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	pullChanMutex sync.Mutex
	_             [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	pushChan      chan flushingBatch[T]
	_             [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	pullChan      chan []T
	_             [CacheLinePadding]byte // makes it 2-3x faster :D, due to prevention of false sharing
	currentBatch  atomic.Pointer[proccessingBatch[T]]
}

type flushingBatch[T any] struct {
	size int
	buf  []T
}

func NewBuffer[T any](batchSize int, queueSize int) *Buffer[T] {

	if queueSize <= 0 {
		panic("queueSize must be greater than 0")
	}

	if queueSize%batchSize != 0 {
		panic("queueSize must be a multiple of batchSize")
	}

	totalBufCount := queueSize/batchSize + 1

	res := &Buffer[T]{
		pushChan: make(chan flushingBatch[T], totalBufCount),
		pullChan: make(chan []T, totalBufCount),
	}

	// add totalBufCount to pullChan
	for i := 0; i < totalBufCount; i++ {
		res.pullChan <- make([]T, batchSize)
	}

	go res.workerLoop()

	return res
}

func (b *Buffer[T]) Close() {
	close(b.pushChan)
}

func (b *Buffer[T]) workerLoop() {

	// TODO: Create ticker to flush buffer every x seconds

	for {
		select {
		case buf := <-b.pushChan:
			// TODO: Determine read length
			slog.Info(fmt.Sprintf("Flushing buffer. nWritten: %d", buf.size))
			// push it back
			b.pullChan <- buf.buf
		}
	}
}

func (b *Buffer[T]) PushOne(item T) {

tryAgain:

	var batch *proccessingBatch[T] = b.currentBatch.Load()
	if batch == nil {
		func() {
			b.pullChanMutex.Lock()
			defer b.pullChanMutex.Unlock()
			batch = b.currentBatch.Load()
			if batch == nil {
				data := <-b.pullChan
				batch = &proccessingBatch[T]{data: data}
				b.currentBatch.Store(batch)
			}
		}()
	}

	writePos := batch.writePos.Add(1) - 1
	if int(writePos) == len(batch.data) {
		for batch.nWritten.Load() != uint64(len(batch.data)) {
			runtime.Gosched() // must wait until all elements are written, i.e. all other goroutines have finished writing
		}
		b.pushChan <- flushingBatch[T]{size: len(batch.data), buf: batch.data}
		b.currentBatch.Store(nil)
		batch.nWritten.Store(0)
		batch.writePos.Store(0)
		goto tryAgain
	} else if int(writePos) > len(batch.data) {
		runtime.Gosched()
		goto tryAgain
	}

	batch.data[writePos] = item
	batch.nWritten.Add(1)
}
