package snail_test_util

import (
	"sync/atomic"
	"time"
)

func Schedule(after time.Duration, f func()) {
	go func() {
		time.Sleep(after)
		f()
	}()
}

func TrueForTimeout(timeout time.Duration) *atomic.Bool {
	res := atomic.Bool{}
	res.Store(true)
	Schedule(timeout, func() { res.Store(false) })
	return &res
}
