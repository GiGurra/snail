package snail_channel

import "sync"

// SnailChannel is a circular buffer with a fixed size.
type SnailChannel[T any] struct {
	data     []T
	writePos int
	readPos  int
	nElems   int
	lock     sync.Mutex
	cond     *sync.Cond
}

func NewSnailChannel[T any](size int) *SnailChannel[T] {
	res := &SnailChannel[T]{data: make([]T, size)}
	res.cond = sync.NewCond(&res.lock)
	return res
}

func (sc *SnailChannel[T]) dataInChannelUnsafe() int {
	// if write pos is after read pos, it's easy
	return sc.nElems
}

func (sc *SnailChannel[T]) freeSpaceUnsafe() int {
	return len(sc.data) - sc.dataInChannelUnsafe()
}

func (sc *SnailChannel[T]) Add(data T) {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	for sc.freeSpaceUnsafe() == 0 {
		sc.cond.Wait()
	}

	sc.data[sc.writePos] = data
	sc.writePos = (sc.writePos + 1) % len(sc.data)
	sc.nElems++

	sc.cond.Broadcast()
}

func (sc *SnailChannel[T]) Pop() T {
	sc.lock.Lock()
	defer sc.lock.Unlock()

	for sc.dataInChannelUnsafe() == 0 {
		sc.cond.Wait()
	}

	res := sc.data[sc.readPos]
	sc.readPos = (sc.readPos + 1) % len(sc.data)
	sc.nElems--

	sc.cond.Broadcast()

	return res
}
