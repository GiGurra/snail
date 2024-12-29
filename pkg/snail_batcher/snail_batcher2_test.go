package snail_batcher

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"github.com/GiGurra/snail/pkg/snail_test_util"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"log/slog"
	"testing"
	"time"
)

func TestPerfOfNewSnailBatcher2_efficientRoutines(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 400_000_000
	batchSize := 10_000 // 10 000 seems to be the sweet spot
	nGoRoutines := 512  // 10   // turns out pulling from multiple goroutines is slower

	nExpectedResults := nItems / batchSize

	resultsChannel := make(chan []int, nExpectedResults)

	t0 := time.Now()
	go func() {

		lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
			batcher := NewSnailBatcher2[int](
				1*time.Minute, // dont want the tickers interfering
				batchSize,
				batchSize*2,
				func(values []int) error {
					resultsChannel <- values
					return nil
				},
			)
			defer batcher.Close()

			for i := 0; i < nItems/nGoRoutines; i++ {
				batcher.Add(i)
			}
			batcher.Flush()
		})

		resultsChannel <- make([]int, nItems%nGoRoutines)

	}()

	slog.Info("waiting for results")

	totalReceived := 0
	for totalReceived < nItems {
		select {
		case items := <-resultsChannel:
			totalReceived += len(items)
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for results")
		}
	}

	if totalReceived != nItems {
		t.Fatalf("unexpected total received %d", totalReceived)
	}

	slog.Info("all results received")

	timeElapsed := time.Since(t0)

	rate := float64(nItems) / timeElapsed.Seconds()

	slog.Info(fmt.Sprintf("Processed %s items in %s", prettyInt3Digits(int64(nItems)), timeElapsed))
	slog.Info(fmt.Sprintf("Rate: %s items/sec", prettyInt3Digits(int64(rate))))
}

func TestPerfOfNewSnailBatcher2_inEfficientRoutines(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	testLength := 1 * time.Second
	batchSize := 1000
	nGoRoutines := 1000

	nReceived := int64(0)
	batcher := NewSnailBatcher2[int](
		1*time.Minute, // dont want the tickers interfering
		batchSize,
		batchSize*5,
		func(values []int) error {
			nReceived += int64(len(values))
			return nil
		},
	)
	defer batcher.Close()

	sendIsOngoing := snail_test_util.TrueForTimeout(testLength)

	t0 := time.Now()

	nSent := lo.Sum(lop.Map(lo.Range(nGoRoutines), func(i int, _ int) int64 {
		nSentThisRoutine := int64(0)
		for sendIsOngoing.Load() {
			batcher.Add(i)
			nSentThisRoutine++
		}
		batcher.Flush()
		return nSentThisRoutine
	}))

	slog.Info("Done sending, measuring how much we have received")

	timeElapsed := time.Since(t0)

	receiveRate := float64(nReceived) / timeElapsed.Seconds()
	sendRate := float64(nSent) / timeElapsed.Seconds()

	slog.Info(fmt.Sprintf("Received %s items in %s", prettyInt3Digits(nReceived), timeElapsed))
	slog.Info(fmt.Sprintf("Receive Rate: %s items/sec", prettyInt3Digits(int64(receiveRate))))

	slog.Info(fmt.Sprintf("Sent %s items in %s", prettyInt3Digits(nSent), timeElapsed))
	slog.Info(fmt.Sprintf("Send Rate: %s items/sec", prettyInt3Digits(int64(sendRate))))
}

func TestNewSnailBatcher2_flushesAfterTimeout(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 1
	batchSize := 10_000
	flushTime := 10 * time.Millisecond

	resultsChannel := make(chan []int, 1)

	batcher := NewSnailBatcher2[int](
		flushTime,
		batchSize,
		batchSize*2,
		func(values []int) error {
			resultsChannel <- values
			return nil
		},
	)
	defer batcher.Close()

	t0 := time.Now()
	go func() {
		batcher.Add(1)
	}()

	slog.Info("waiting for results")

	totalReceived := 0
	for totalReceived < nItems {
		select {
		case items := <-resultsChannel:
			totalReceived += len(items)
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for results")
		}
	}

	if totalReceived != nItems {
		t.Fatalf("unexpected total received %d", totalReceived)
	}

	slog.Info("all results received")

	timeElapsed := time.Since(t0)

	if timeElapsed > 10*flushTime {
		t.Fatalf("expected flush after timeout")
	}
}

func TestNewSnailBatcher2_flushesAfterTimeout1s(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 1
	batchSize := 10_000
	flushTime := 1 * time.Second

	resultsChannel := make(chan []int, 1)

	batcher := NewSnailBatcher2[int](
		flushTime,
		batchSize,
		batchSize*2,
		func(values []int) error {
			resultsChannel <- values
			return nil
		},
	)
	defer batcher.Close()

	t0 := time.Now()
	go func() {
		batcher.Add(1)
	}()

	slog.Info("waiting for results")

	totalReceived := 0
	for totalReceived < nItems {
		select {
		case items := <-resultsChannel:
			totalReceived += len(items)
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for results")
		}
	}

	if totalReceived != nItems {
		t.Fatalf("unexpected total received %d", totalReceived)
	}

	slog.Info("all results received")

	timeElapsed := time.Since(t0)

	if timeElapsed < 500*time.Millisecond {
		t.Fatalf("Received too early")
	}

	if timeElapsed > 2*flushTime {
		t.Fatalf("Received too late")
	}
}

func TestNewSnailBatcher2_manualFlush(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 1
	batchSize := 10_000
	flushTime := 1 * time.Second

	resultsChannel := make(chan []int, 1)

	batcher := NewSnailBatcher2[int](
		flushTime,
		batchSize,
		batchSize*2,
		func(values []int) error {
			resultsChannel <- values
			return nil
		},
	)
	defer batcher.Close()

	t0 := time.Now()
	go func() {
		batcher.Add(1)
		batcher.Flush()
	}()

	slog.Info("waiting for results")

	totalReceived := 0
	for totalReceived < nItems {
		select {
		case items := <-resultsChannel:
			totalReceived += len(items)
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for results")
		}
	}

	if totalReceived != nItems {
		t.Fatalf("unexpected total received %d", totalReceived)
	}

	slog.Info("all results received")

	timeElapsed := time.Since(t0)

	if timeElapsed > 500*time.Millisecond {
		t.Fatalf("Received too late")
	}
}
