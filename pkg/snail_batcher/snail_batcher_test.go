package snail_batcher

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"log/slog"
	"testing"
	"time"
)

func TestPerfOfNewSnailBatcher(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 40_000_000
	batchSize := 10_000 // 10 000 seems to be the sweet spot
	nGoRoutines := 1    // 10   // turns out pulling from multiple goroutines is slower

	nExpectedResults := nItems / batchSize

	resultsChannel := make(chan []int, nExpectedResults)

	t0 := time.Now()
	go func() {

		batcher := NewSnailBatcher[int](
			1*time.Minute, // dont want the tickers interfering
			batchSize,
			batchSize*2,
			func(values []int) error {
				resultsChannel <- values
				return nil
			},
		)
		defer batcher.Close()

		lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
			for i := 0; i < nItems/nGoRoutines; i++ {
				batcher.Add(i)
			}
		})

		batcher.Flush()

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

func TestPerfOfNewSnailBatcher_efficientRoutines(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 400_000_000
	batchSize := 10_000 // 10 000 seems to be the sweet spot
	nGoRoutines := 512  // 10   // turns out pulling from multiple goroutines is slower

	nExpectedResults := nItems / batchSize

	resultsChannel := make(chan []int, nExpectedResults)

	t0 := time.Now()
	go func() {

		lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
			batcher := NewSnailBatcher[int](
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

func TestPerfOfNewSnailBatcher_inEfficientRoutines(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 1_000_000
	batchSize := 1000 // 10 000 seems to be the sweet spot
	nGoRoutines := 1000

	nExpectedResults := nItems / batchSize

	resultsChannel := make(chan []int, nExpectedResults)

	batcher := NewSnailBatcher[int](
		1*time.Minute, // dont want the tickers interfering
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

		lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
			for i := 0; i < nItems/nGoRoutines; i++ {
				batcher.Add(i)
			}
		})

		batcher.Flush()

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

func TestNewSnailBatcher_flushesAfterTimeout(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 1
	batchSize := 10_000
	flushTime := 10 * time.Millisecond

	resultsChannel := make(chan []int, 1)

	batcher := NewSnailBatcher[int](
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

func TestNewSnailBatcher_flushesAfterTimeout1s(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 1
	batchSize := 10_000
	flushTime := 1 * time.Second

	resultsChannel := make(chan []int, 1)

	batcher := NewSnailBatcher[int](
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

func TestNewSnailBatcher_manualFlush(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 1
	batchSize := 10_000
	flushTime := 1 * time.Second

	resultsChannel := make(chan []int, 1)

	batcher := NewSnailBatcher[int](
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

var prettyPrinter = message.NewPrinter(language.English)

func prettyInt3Digits(n int64) string {
	return prettyPrinter.Sprintf("%d", n)
}
