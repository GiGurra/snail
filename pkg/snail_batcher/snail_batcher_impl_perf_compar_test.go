package snail_batcher

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestBuffer(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	numProducers := 1000
	totalElements := 640_000_000
	elementsPerProducer := totalElements / numProducers

	buf := NewSnailBatcherNonBlockingPrototype[int64](totalElements/10, totalElements/5)

	// Example of multiple producers
	var wg sync.WaitGroup
	wg.Add(numProducers)

	start := time.Now()

	for i := 0; i < numProducers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < elementsPerProducer; j++ {
				buf.PushOne(int64(j))
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)
	opsPerSec := float64(totalElements) / elapsed.Seconds()
	fmt.Printf("Operations per second: %.2f M\n", opsPerSec/1_000_000)
}

func TestBufferWithMutex(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	numProducers := 1000
	totalElements := 640_000_000
	elementsPerProducer := totalElements / numProducers

	buf := NewBufferMutexBasedPrototype[int64](totalElements / 10)

	// Example of multiple producers
	var wg sync.WaitGroup
	wg.Add(numProducers)

	start := time.Now()

	for i := 0; i < numProducers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < elementsPerProducer; j++ {
				buf.PushOne(int64(j))
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)
	opsPerSec := float64(totalElements) / elapsed.Seconds()
	fmt.Printf("Operations per second: %.2f M\n", opsPerSec/1_000_000)
}

func TestSnailBatcher_AddWithIdiotMutexPerf(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	numProducers := 1000
	totalElements := 640_000_000
	elementsPerProducer := totalElements / numProducers

	buf := NewSnailBatcher[int64](
		totalElements/10,
		2*totalElements/10,
		true,
		1*time.Minute,
		func(batch []int64) error {
			slog.Info(fmt.Sprintf("Flushing buffer. nWritten: %d", len(batch)))
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
