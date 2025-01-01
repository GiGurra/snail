package snail_test_util

import (
	"golang.org/x/text/language"
	"golang.org/x/text/message"
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

var prettyPrinter = message.NewPrinter(language.English)

func PrettyInt3Digits(n int64) string {
	return prettyPrinter.Sprintf("%d", n)
}
