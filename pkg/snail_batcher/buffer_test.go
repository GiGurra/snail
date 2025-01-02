package snail_batcher

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"sync"
	"testing"
	"time"
)

func TestBuffer(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	numProducers := 512
	totalElements := 640_000_000
	elementsPerProducer := totalElements / numProducers

	buf := NewBuffer(totalElements / 10)

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

	numProducers := 512
	totalElements := 640_000_000
	elementsPerProducer := totalElements / numProducers

	buf := NewBufferWithMutex(totalElements / 10)

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
