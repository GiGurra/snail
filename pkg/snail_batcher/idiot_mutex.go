package snail_batcher

import (
	"sync/atomic"
)

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

type idiotMutex struct {
	noCopy
	value atomic.Bool
}

func (m *idiotMutex) TryLock() bool {
	return m.value.CompareAndSwap(false, true)
}

func (m *idiotMutex) Unlock() {
	m.value.Store(false)
}
