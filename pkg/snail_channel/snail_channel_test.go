package snail_channel

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_test_util"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewSnailChannel(t *testing.T) {
	size := 10
	nElemsToTest := 500
	sc := NewSnailChannel[int](size)
	go func() {
		for i := 0; i < nElemsToTest; i++ {
			sc.Add(i)
		}
	}()

	for i := 0; i < nElemsToTest; i++ {
		elem := sc.Pop()
		if elem != i {
			t.Fatalf("Expected %d, got %d", i, elem)
		}
	}
}

func TestNewSnailChannelCountdata(t *testing.T) {
	size := 10
	sc := NewSnailChannel[int](size)
	for i := 0; i < 5; i++ {
		sc.Add(i)
	}

	if sc.dataInChannelUnsafe() != 5 {
		t.Fatalf("Expected %d, got %d", 5, sc.dataInChannelUnsafe())
	}

	// add 5 more
	for i := 0; i < 5; i++ {
		sc.Add(i)
	}

	if sc.dataInChannelUnsafe() != 10 {
		t.Fatalf("Expected %d, got %d", 10, sc.dataInChannelUnsafe())
	}

	// pop 5
	for i := 0; i < 5; i++ {
		sc.Pop()
	}

	if sc.dataInChannelUnsafe() != 5 {
		t.Fatalf("Expected %d, got %d", 5, sc.dataInChannelUnsafe())
	}

	// add 5 more
	for i := 0; i < 5; i++ {
		sc.Add(i)
	}

	if sc.dataInChannelUnsafe() != 10 {
		t.Fatalf("Expected %d, got %d", 10, sc.dataInChannelUnsafe())
	}
}

func TestSnailChannel_benchmark_performance(t *testing.T) {

	testLength := 1 * time.Second
	size := 1

	sc := NewSnailChannel[int](size)
	keepRunningReader := snail_test_util.TrueForTimeout(testLength)
	keepRunningWriter := atomic.Bool{}
	keepRunningWriter.Store(true)

	go func() {
		for keepRunningWriter.Load() {
			sc.Add(0)
		}
	}()

	nElementsTransferred := 0
	t0 := time.Now()
	for keepRunningReader.Load() {
		sc.Pop()
		nElementsTransferred++
	}
	elapsed := time.Since(t0)
	keepRunningWriter.Store(false)

	opsPerSec := float64(nElementsTransferred) / elapsed.Seconds()

	slog.Info(fmt.Sprintf("Operations per second: %.2f M", opsPerSec/1_000_000))
}

func TestSnailChannel_benchmark_regular_channels(t *testing.T) {

	testLength := 1 * time.Second
	size := 0

	sc := make(chan int, size)
	keepRunningReader := snail_test_util.TrueForTimeout(testLength)
	keepRunningWriter := atomic.Bool{}
	keepRunningWriter.Store(true)

	go func() {
		for keepRunningWriter.Load() {
			sc <- 0
		}
	}()

	nElementsTransferred := 0
	t0 := time.Now()
	for keepRunningReader.Load() {
		<-sc
		nElementsTransferred++
	}
	elapsed := time.Since(t0)
	keepRunningWriter.Store(false)

	opsPerSec := float64(nElementsTransferred) / elapsed.Seconds()

	slog.Info(fmt.Sprintf("Operations per second: %.2f M", opsPerSec/1_000_000))
}
