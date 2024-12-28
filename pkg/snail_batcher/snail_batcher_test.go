package snail_batcher

import (
	"log/slog"
	"testing"
	"time"
)

func TestNewSnailBatcher(t *testing.T) {

	nItems := 10_000_000
	batchSize := 10_000

	nExpectedResults := nItems / batchSize

	resultsChannel := make(chan []int, nExpectedResults)

	batcher := NewSnailBatcher[int](
		1*time.Second,
		batchSize,
		func(values []int) error {
			resultsChannel <- values
			return nil
		},
	)
	defer batcher.Close()

	go func() {
		for i := 0; i < nItems; i++ {
			batcher.Add(i)
		}
	}()

	for i := 0; i < nExpectedResults; i++ {
		select {
		case items := <-resultsChannel:
			if len(items) != batchSize {
				t.Fatalf("unexpected batch size %d", len(items))
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for result %d", i)
		}
	}

	slog.Info("all results received")
}
