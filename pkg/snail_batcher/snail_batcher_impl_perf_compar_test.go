package snail_batcher

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTheoreticalLimitOfMutex_oneThread(t *testing.T) {

	nAddsLimit := 10_000_000
	nGoRoutines := 1

	counter := 0
	mutex := sync.Mutex{}

	t0 := time.Now()
	lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
		for i := 0; i < nAddsLimit/nGoRoutines; i++ {
			mutex.Lock()
			counter += 1
			mutex.Unlock()
		}
	})

	elapsed := time.Since(t0)

	opsPerSec := float64(counter) / elapsed.Seconds()

	slog.Info(fmt.Sprintf("Operations per second: %.2f M", opsPerSec/1_000_000))

}

func TestTheoreticalLimitOfMutex_manyThreads(t *testing.T) {

	nAddsLimit := 10_000_000
	nGoRoutines := 10_000

	counter := 0
	mutex := sync.Mutex{}

	t0 := time.Now()
	lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
		for i := 0; i < nAddsLimit/nGoRoutines; i++ {
			mutex.Lock()
			counter += 1
			mutex.Unlock()
		}
	})

	elapsed := time.Since(t0)

	opsPerSec := float64(counter) / elapsed.Seconds()

	slog.Info(fmt.Sprintf("Operations per second: %.2f M", opsPerSec/1_000_000))

}

type spinLock struct {
	mutex sync.Mutex
}

func (m *spinLock) Lock() {
	for !m.mutex.TryLock() {
		runtime.Gosched()
	}
}

func (m *spinLock) Unlock() {
	m.mutex.Unlock()
}

func TestTheoreticalLimitOfSpinLock_oneThread(t *testing.T) {

	nAddsLimit := 100_000_000
	nGoRoutines := 1

	counter := 0
	mutex := spinLock{}

	t0 := time.Now()
	lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
		for i := 0; i < nAddsLimit/nGoRoutines; i++ {
			mutex.Lock()
			counter += 1
			mutex.Unlock()
		}
	})

	elapsed := time.Since(t0)

	opsPerSec := float64(counter) / elapsed.Seconds()

	slog.Info(fmt.Sprintf("Operations per second: %.2f M", opsPerSec/1_000_000))

}

func TestTheoreticalLimitOfSpinLock_manyThreads(t *testing.T) {

	nAddsLimit := 100_000_000
	nGoRoutines := 10_000

	counter := 0
	mutex := spinLock{}

	t0 := time.Now()
	lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
		for i := 0; i < nAddsLimit/nGoRoutines; i++ {
			mutex.Lock()
			counter += 1
			mutex.Unlock()
		}
	})

	elapsed := time.Since(t0)

	opsPerSec := float64(counter) / elapsed.Seconds()

	slog.Info(fmt.Sprintf("Operations per second: %.2f M", opsPerSec/1_000_000))

}

func TestTheoreticalLimitsOfChannels_oneThread(t *testing.T) {

	nAddsLimit := 1_000_000
	nGoRoutines := 1

	// Check that nAddsLimit is divisible by nGoRoutines
	if //goland:noinspection GoBoolExpressions
	nAddsLimit%nGoRoutines != 0 {
		t.Fatalf("nAddsLimit must be divisible by nGoRoutines")
	}

	ch := make(chan struct{}, 100_000)

	t0 := time.Now()
	go func() {
		lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
			for i := 0; i < nAddsLimit/nGoRoutines; i++ {
				ch <- struct{}{}
			}
		})
	}()

	counter := 0
	for counter < nAddsLimit {
		<-ch
		counter++
	}

	elapsed := time.Since(t0)

	opsPerSec := float64(counter) / elapsed.Seconds()

	slog.Info(fmt.Sprintf("Operations per second: %.2f M", opsPerSec/1_000_000))
}

func TestTheoreticalLimitsOfChannels_manyThreads(t *testing.T) {

	nAddsLimit := 1_000_000
	nGoRoutines := 10_000

	// Check that nAddsLimit is divisible by nGoRoutines
	if //goland:noinspection GoBoolExpressions
	nAddsLimit%nGoRoutines != 0 {
		t.Fatalf("nAddsLimit must be divisible by nGoRoutines")
	}

	ch := make(chan struct{}, 100_000)

	t0 := time.Now()
	go func() {
		lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
			for i := 0; i < nAddsLimit/nGoRoutines; i++ {
				ch <- struct{}{}
			}
		})
	}()

	counter := 0
	for counter < nAddsLimit {
		<-ch
		counter++
	}

	elapsed := time.Since(t0)

	opsPerSec := float64(counter) / elapsed.Seconds()

	slog.Info(fmt.Sprintf("Operations per second: %.2f M", opsPerSec/1_000_000))
}

func TestTheoreticalLimitOfAtomics_oneThread(t *testing.T) {

	nAddsLimit := 10_000_000
	nGoRoutines := 1

	counter := atomic.Uint64{}

	t0 := time.Now()
	lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
		for i := 0; i < nAddsLimit/nGoRoutines; i++ {
			counter.Add(1)
		}
	})

	elapsed := time.Since(t0)

	opsPerSec := float64(counter.Load()) / elapsed.Seconds()

	slog.Info(fmt.Sprintf("Operations per second: %.2f M", opsPerSec/1_000_000))

}

func TestTheoreticalLimitOfAtomics_manyThreads(t *testing.T) {

	nAddsLimit := 10_000_000
	nGoRoutines := 10_000

	counter := atomic.Uint64{}

	t0 := time.Now()
	lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
		for i := 0; i < nAddsLimit/nGoRoutines; i++ {
			counter.Add(1)
		}
	})

	elapsed := time.Since(t0)

	opsPerSec := float64(counter.Load()) / elapsed.Seconds()

	slog.Info(fmt.Sprintf("Operations per second: %.2f M", opsPerSec/1_000_000))

}

func TestSnailBatcherNonBlockingPrototypePerformance_oneThread(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	numProducers := 1
	totalElements := 60_400_000
	elementsPerProducer := totalElements / numProducers

	buf := NewSnailBatcherNonBlockingPrototype[int64](
		totalElements/10,
		totalElements/5,
		1*time.Minute,
		func(batch []int64) error {
			//slog.Info(fmt.Sprintf("Flushing buffer. nWritten: %d", len(batch)))
			return nil
		},
	)

	// Example of multiple producers
	var wg sync.WaitGroup
	wg.Add(numProducers)

	start := time.Now()

	for i := 0; i < numProducers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < elementsPerProducer; j++ {
				buf.Add(int64(j))
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)
	opsPerSec := float64(totalElements) / elapsed.Seconds()
	fmt.Printf("Operations per second: %.2f M\n", opsPerSec/1_000_000)
}

func TestSnailBatcherNonBlockingPrototypePerformance_manyThreads(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	numProducers := 1000
	totalElements := 60_400_000
	elementsPerProducer := totalElements / numProducers

	buf := NewSnailBatcherNonBlockingPrototype[int64](
		totalElements/10,
		totalElements/5,
		1*time.Minute,
		func(batch []int64) error {
			//slog.Info(fmt.Sprintf("Flushing buffer. nWritten: %d", len(batch)))
			return nil
		},
	)

	// Example of multiple producers
	var wg sync.WaitGroup
	wg.Add(numProducers)

	start := time.Now()

	for i := 0; i < numProducers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < elementsPerProducer; j++ {
				buf.Add(int64(j))
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)
	opsPerSec := float64(totalElements) / elapsed.Seconds()
	fmt.Printf("Operations per second: %.2f M\n", opsPerSec/1_000_000)
}

func TestSnailBatcherPerformance_oneThread(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	numProducers := 1
	totalElements := 60_400_000
	elementsPerProducer := totalElements / numProducers

	buf := NewSnailBatcher[int64](
		totalElements/10,
		totalElements/5,
		1*time.Minute,
		func(batch []int64) error {
			//slog.Info(fmt.Sprintf("Flushing buffer. nWritten: %d", len(batch)))
			return nil
		},
	)

	// Example of multiple producers
	var wg sync.WaitGroup
	wg.Add(numProducers)

	start := time.Now()

	for i := 0; i < numProducers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < elementsPerProducer; j++ {
				buf.Add(int64(j))
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)
	opsPerSec := float64(totalElements) / elapsed.Seconds()
	fmt.Printf("Operations per second: %.2f M\n", opsPerSec/1_000_000)
}

func TestSnailBatcherPerformance_manyThreads(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	numProducers := 1000
	totalElements := 60_400_000
	elementsPerProducer := totalElements / numProducers

	buf := NewSnailBatcher[int64](
		totalElements/10,
		totalElements/5,
		1*time.Minute,
		func(batch []int64) error {
			//slog.Info(fmt.Sprintf("Flushing buffer. nWritten: %d", len(batch)))
			return nil
		},
	)

	// Example of multiple producers
	var wg sync.WaitGroup
	wg.Add(numProducers)

	start := time.Now()

	for i := 0; i < numProducers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < elementsPerProducer; j++ {
				buf.Add(int64(j))
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)
	opsPerSec := float64(totalElements) / elapsed.Seconds()
	fmt.Printf("Operations per second: %.2f M\n", opsPerSec/1_000_000)
}
