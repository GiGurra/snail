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

func TestNewSnailBatcher(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 40_000_000
	batchSize := 10_000 // 10 000 seems to be the sweet spot
	nGoRoutines := 1    // turns out pulling from multiple goroutines is slower

	nExpectedResults := nItems / batchSize

	resultsChannel := make(chan []int, nExpectedResults)

	batcher := NewSnailBatcher[int](
		10*time.Millisecond, // simulate a 10ms window
		batchSize,
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

var prettyPrinter = message.NewPrinter(language.English)

func prettyInt3Digits(n int64) string {
	return prettyPrinter.Sprintf("%d", n)
}
