package snail_batcher

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"github.com/GiGurra/snail/pkg/snail_test_util"
	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"log/slog"
	"slices"
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
			batchSize,
			batchSize*2,
			true,
			1*time.Minute, // dont want the tickers interfering
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

func TestAddMany(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 100
	batchSize := 10 // 10 000 seems to be the sweet spot

	// ensure nItems is a multiple of batchSize
	if //goland:noinspection GoBoolExpressions
	nItems%batchSize != 0 {
		t.Fatalf("nItems must be a multiple of batchSize")
	}

	itemsToAdd := make([]int, nItems)
	for i := 0; i < nItems; i++ {
		itemsToAdd[i] = i
	}

	resultsChannel := make(chan []int, nItems/batchSize)

	batcher := NewSnailBatcher[int](
		batchSize,
		batchSize*2,
		true,
		1*time.Minute, // dont want the tickers interfering
		func(values []int) error {
			slog.Info(fmt.Sprintf("Received %d items", len(values)))
			resultsChannel <- slices.Clone(values)
			return nil
		},
	)

	batcher.AddMany(itemsToAdd)

	slog.Info("waiting for results")

	receivedItems := make([]int, 0, nItems)

	for len(receivedItems) < nItems {
		select {
		case items := <-resultsChannel:
			if len(items) != batchSize {
				t.Fatalf("unexpected batch size %d", len(items))
			}
			for _, item := range items {
				receivedItems = append(receivedItems, item)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout waiting for results")
		}
	}

	if len(receivedItems) != nItems {
		t.Fatalf("unexpected total received %d", len(receivedItems))
	}

	if diff := cmp.Diff(itemsToAdd, receivedItems); diff != "" {
		t.Fatalf("unexpected items received: %s", diff)
	}

	slog.Info("all results received")

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
				batchSize,
				batchSize*2,
				true,
				1*time.Minute, // dont want the tickers interfering
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

	testLength := 1 * time.Second
	batchSize := 1000
	nGoRoutines := 100

	nReceived := int64(0)
	batcher := NewSnailBatcher[int](
		batchSize,
		batchSize*5,
		true,
		1*time.Minute, // dont want the tickers interfering
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

func TestNewSnailBatcher_flushesAfterTimeout(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 1
	batchSize := 10_000
	flushTime := 10 * time.Millisecond

	resultsChannel := make(chan []int, 1)

	batcher := NewSnailBatcher[int](
		batchSize,
		batchSize*2,
		true,
		flushTime,
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
		batchSize,
		batchSize*2,
		true,
		flushTime,
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
		batchSize,
		batchSize*2,
		true,
		flushTime,
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
