package snail_batcher

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"github.com/GiGurra/snail/pkg/snail_test_util"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"log/slog"
	"math/rand"
	"slices"
	"testing"
	"time"
)

func TestPerfOfNewSnailBatcherNonBlockingPrototype(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 40_000_000
	batchSize := 10_000 // 10 000 seems to be the sweet spot
	nGoRoutines := 1    // 10   // turns out pulling from multiple goroutines is slower

	nExpectedResults := nItems / batchSize

	resultsChannel := make(chan []int, nExpectedResults)

	t0 := time.Now()
	go func() {

		batcher := NewSnailBatcherNonBlockingPrototype[int](
			batchSize,
			batchSize*2,
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

func TestAddManyCorrectnessNonBlockingPrototype_simple(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 20
	batchSize := 5

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

	batcher := NewSnailBatcherNonBlockingPrototype[int](
		batchSize,
		batchSize*2,
		1*time.Minute, // dont want the tickers interfering
		func(values []int) error {
			resultsChannel <- slices.Clone(values)
			return nil
		},
	)

	// Add in some semi random fashion
	for itemsAdded := 0; itemsAdded < nItems; {
		maxThisStep := min(3*batchSize, nItems-itemsAdded)
		nToAddThisStep := rand.Intn(maxThisStep + 1)
		//if rand.Float64() < 0.5 { // bulk
		batcher.AddMany(itemsToAdd[itemsAdded : itemsAdded+nToAddThisStep])
		itemsAdded += nToAddThisStep
		//} else { // add one at a time
		//	for i := 0; i < nToAddThisStep; i++ {
		//		batcher.Add(itemsToAdd[itemsAdded])
		//		itemsAdded++
		//	}
		//}
	}

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

	if !slices.Equal(itemsToAdd, receivedItems) {
		t.Fatalf("unexpected items received")
	}

	slog.Info("all results received")

}

func TestAddManyCorrectnessNonBlockingPrototype(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 10_000_000
	batchSize := 100

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

	batcher := NewSnailBatcherNonBlockingPrototype[int](
		batchSize,
		batchSize*2,
		1*time.Minute, // dont want the tickers interfering
		func(values []int) error {
			resultsChannel <- slices.Clone(values)
			return nil
		},
	)

	// Add in some semi random fashion
	for itemsAdded := 0; itemsAdded < nItems; {
		maxThisStep := min(3*batchSize, nItems-itemsAdded)
		nToAddThisStep := rand.Intn(maxThisStep + 1)
		if rand.Float64() < 0.5 { // bulk
			batcher.AddMany(itemsToAdd[itemsAdded : itemsAdded+nToAddThisStep])
			itemsAdded += nToAddThisStep
		} else { // add one at a time
			for i := 0; i < nToAddThisStep; i++ {
				batcher.Add(itemsToAdd[itemsAdded])
				itemsAdded++
			}
		}
	}

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

	if !slices.Equal(itemsToAdd, receivedItems) {
		t.Fatalf("unexpected items received")
	}

	slog.Info("all results received")

}

func TestAddManyPerformanceNonBlockingPrototype(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 100_000_000 // increase to more when benchmarking. 100m is too little
	batchSize := 1000
	nGoRoutines := 12

	// ensure nItems is a multiple of batchSize
	if //goland:noinspection GoBoolExpressions
	nItems%batchSize != 0 {
		t.Fatalf("nItems must be a multiple of batchSize")
	}

	slog.Info("Preparing test data")
	testDataSource := make([]int, batchSize)

	slog.Info("Starting test")
	elapsedTimes := make([]time.Duration, nGoRoutines)
	lop.ForEach(lo.Range(nGoRoutines), func(iGR int, _ int) {
		doneSignal := make(chan struct{})
		receivedCount := 0

		batcher := NewSnailBatcherNonBlockingPrototype[int](
			batchSize,
			batchSize*2,
			1*time.Minute, // dont want the tickers interfering
			func(values []int) error {
				//slog.Info(fmt.Sprintf("Received response. n: %d", len(values)))
				receivedCount += len(values)
				if receivedCount == nItems {
					close(doneSignal)
				}
				return nil
			},
		)

		t0 := time.Now()

		for itemsAdded := 0; itemsAdded < nItems; {
			nToAddThisStep := max(1, min(2*batchSize/3, nItems-itemsAdded))
			batcher.AddMany(testDataSource[:nToAddThisStep])
			itemsAdded += nToAddThisStep
		}

		// wait for the done signal
		<-doneSignal

		elapsedTimes[iGR] = time.Since(t0)
	})

	elapsed := time.Duration(lo.Sum(lo.Map(elapsedTimes, func(d time.Duration, _ int) int64 {
		return d.Nanoseconds()
	})) / int64(nGoRoutines))

	nItemsTot := int64(nItems) * int64(nGoRoutines)
	rate := float64(nItemsTot) / elapsed.Seconds()

	slog.Info(fmt.Sprintf("Processed %s items in %s", prettyInt3Digits(nItemsTot), elapsed))
	slog.Info(fmt.Sprintf("Rate: %s items/sec", prettyInt3Digits(int64(rate))))

}

func TestPerfOfNewSnailBatcherNonBlockingPrototype_efficientRoutines(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 400_000_000
	batchSize := 10_000 // 10 000 seems to be the sweet spot
	nGoRoutines := 512  // 10   // turns out pulling from multiple goroutines is slower

	nExpectedResults := nItems / batchSize

	resultsChannel := make(chan []int, nExpectedResults)

	t0 := time.Now()
	go func() {

		lop.ForEach(lo.Range(nGoRoutines), func(_ int, _ int) {
			batcher := NewSnailBatcherNonBlockingPrototype[int](
				batchSize,
				batchSize*2,
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

func TestPerfOfNewSnailBatcherNonBlockingPrototype_inEfficientRoutines(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	testLength := 1 * time.Second
	batchSize := 1000
	nGoRoutines := 100

	nReceived := int64(0)
	batcher := NewSnailBatcherNonBlockingPrototype[int](
		batchSize,
		batchSize*5,
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

func TestNewSnailBatcherNonBlockingPrototype_flushesAfterTimeout(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 1
	batchSize := 10_000
	flushTime := 10 * time.Millisecond

	resultsChannel := make(chan []int, 1)

	batcher := NewSnailBatcherNonBlockingPrototype[int](
		batchSize,
		batchSize*2,
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

func TestNewSnailBatcherNonBlockingPrototype_flushesAfterTimeout1s(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 1
	batchSize := 10_000
	flushTime := 1 * time.Second

	resultsChannel := make(chan []int, 1)

	batcher := NewSnailBatcherNonBlockingPrototype[int](
		batchSize,
		batchSize*2,
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

func TestNewSnailBatcherNonBlockingPrototype_manualFlush(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("text", "info", false)

	nItems := 1
	batchSize := 10_000
	flushTime := 1 * time.Second

	resultsChannel := make(chan []int, 1)

	batcher := NewSnailBatcherNonBlockingPrototype[int](
		batchSize,
		batchSize*2,
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
