package snail_tcp_reqrep

import (
	"encoding/json"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_batcher"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"github.com/GiGurra/snail/pkg/snail_parser"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClient_SendAndRespondWithJson(t *testing.T) {
	snail_logging.ConfigureDefaultLogger("text", "info", false)

	slog.Info("TestNewServer_SendAndRespondWithJson")

	reqCodec := snail_parser.NewJsonLinesCodec[*requestStruct]()
	respCodec := snail_parser.NewJsonLinesCodec[*responseStruct]()

	server, err := NewServer[*requestStruct, *responseStruct](
		func() ServerConnHandler[*requestStruct, *responseStruct] {
			return func(req *requestStruct, repFunc func(resp *responseStruct) error) error {

				if repFunc == nil {
					slog.Warn("Client disconnected")
					return nil
				}

				slog.Info("Server received request", slog.String("msg", req.Msg))
				return repFunc(&responseStruct{Msg: "Hello from server"})
			}
		},
		nil,
		reqCodec.Parser,
		respCodec.Writer,
		nil,
	)

	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	defer server.Close()

	responseChan := make(chan *responseStruct)

	respHandler := func(resp *responseStruct, status ClientStatus) error {

		if resp == nil {
			slog.Warn("Server disconnected")
			return nil
		}

		slog.Info("Client received response", slog.String("msg", resp.Msg))
		responseChan <- resp
		return nil
	}

	client, err := NewClient[*requestStruct, *responseStruct](
		"localhost",
		server.Port(),
		nil,
		respHandler,
		reqCodec.Writer,
		respCodec.Parser,
	)

	if err != nil {
		t.Fatalf("error creating client: %v", err)
	}

	defer client.Close()

	err = client.Send(&requestStruct{Msg: "Hello from client"})
	if err != nil {
		t.Fatalf("error sending request: %v", err)
	}

	select {
	case resp := <-responseChan:
		if resp.Msg != "Hello from server" {
			t.Fatalf("expected response msg 'Hello from server', got '%s'", resp.Msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for response")
	}

}

func TestNewClient_SendAndRespondWithJson_1s_naive_performance(t *testing.T) {
	snail_logging.ConfigureDefaultLogger("text", "info", false)

	slog.Info("TestNewServer_SendAndRespondWithJson")

	testLength := 1 * time.Second

	reqCodec := snail_parser.NewJsonLinesCodec[*requestStruct]()
	respCodec := snail_parser.NewJsonLinesCodec[*responseStruct]()

	server, err := NewServer[*requestStruct, *responseStruct](
		func() ServerConnHandler[*requestStruct, *responseStruct] {
			return func(req *requestStruct, repFunc func(resp *responseStruct) error) error {

				if repFunc == nil {
					slog.Warn("Client disconnected")
					return nil
				}

				//slog.Info("Server received request", slog.String("msg", req.Msg))
				return repFunc(&responseStruct{Msg: "Hello from server"})
			}
		},
		nil,
		reqCodec.Parser,
		respCodec.Writer,
		nil,
	)

	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	defer server.Close()

	responseChan := make(chan *responseStruct)

	respHandler := func(resp *responseStruct, status ClientStatus) error {

		if status == ClientStatusDisconnected {
			slog.Warn("Server disconnected")
			return nil
		}

		//slog.Info("Client received response", slog.String("msg", resp.Msg))
		responseChan <- resp
		return nil
	}

	client, err := NewClient[*requestStruct, *responseStruct](
		"localhost",
		server.Port(),
		nil,
		respHandler,
		reqCodec.Writer,
		respCodec.Parser,
	)

	if err != nil {
		t.Fatalf("error creating client: %v", err)
	}

	defer client.Close()

	nReqResps := 0
	t0 := time.Now()
	for time.Since(t0) < testLength {

		err = client.Send(&requestStruct{Msg: "Hello from client"})
		if err != nil {
			t.Fatalf("error sending request: %v", err)
		}

		select {
		case resp := <-responseChan:
			if resp.Msg != "Hello from server" {
				t.Fatalf("expected response msg 'Hello from server', got '%s'", resp.Msg)
			}
			nReqResps++
		case <-time.After(1 * time.Second):
			t.Fatalf("timeout waiting for response")
		}
	}

	slog.Info("Sent and received", slog.Int("nReqResps", nReqResps))

}

func TestNewClient_SendAndRespondWithInts_1s_naive_performance(t *testing.T) {
	snail_logging.ConfigureDefaultLogger("text", "info", false)

	slog.Info("TestNewServer_SendAndRespondWithInts")

	testLength := 1 * time.Second

	codec := snail_parser.NewInt32Codec()

	server, err := NewServer[int32, int32](
		func() ServerConnHandler[int32, int32] {
			return func(req int32, repFunc func(resp int32) error) error {

				if repFunc == nil {
					slog.Warn("Client disconnected")
					return nil
				}

				//slog.Info("Server received request", slog.String("msg", req.Msg))
				return repFunc(0)
			}
		},
		nil,
		codec.Parser,
		codec.Writer,
		nil,
	)

	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	defer server.Close()

	responseChan := make(chan int32, 100)

	respHandler := func(resp int32, status ClientStatus) error {

		if status == ClientStatusDisconnected {
			slog.Warn("Server disconnected")
			return nil
		}

		//slog.Info("Client received response", slog.String("msg", resp.Msg))
		responseChan <- resp
		return nil
	}

	client, err := NewClient[int32, int32](
		"localhost",
		server.Port(),
		nil,
		respHandler,
		codec.Writer,
		codec.Parser,
	)

	if err != nil {
		t.Fatalf("error creating client: %v", err)
	}

	defer client.Close()

	nReqResps := 0
	t0 := time.Now()
	for time.Since(t0) < testLength {

		err = client.Send(0)
		if err != nil {
			t.Fatalf("error sending request: %v", err)
		}

		select {
		case resp := <-responseChan:
			if resp != 0 {
				t.Fatalf("expected response msg 0, got %d", resp)
			}
			nReqResps++
		case <-time.After(1 * time.Second):
			t.Fatalf("timeout waiting for response")
		}
	}

	slog.Info("Sent and received", slog.Int("nReqResps", nReqResps))

}

func TestNewClient_SendAndRespondWithInts_1s_naive_performance_multiple_goroutines(t *testing.T) {
	snail_logging.ConfigureDefaultLogger("text", "info", false)

	slog.Info("TestNewServer_SendAndRespondWithInts")

	testLength := 1 * time.Second
	nGoRoutines := 512

	codec := snail_parser.NewInt32Codec()

	server, err := NewServer[int32, int32](
		func() ServerConnHandler[int32, int32] {
			return func(req int32, repFunc func(resp int32) error) error {

				if repFunc == nil {
					//slog.Warn("Client disconnected")
					return nil
				}

				//slog.Info("Server received request", slog.String("msg", req.Msg))
				return repFunc(req)
			}
		},
		nil,
		codec.Parser,
		codec.Writer,
		nil,
	)

	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	defer server.Close()

	clients := make([]*SnailClient[int32, int32], nGoRoutines)
	t0 := time.Now()

	sums := make([]int64, nGoRoutines)
	nReqResps := atomic.Int64{}

	wgWriters := sync.WaitGroup{}
	wgWriters.Add(nGoRoutines)
	respHandler := func(resp int32, status ClientStatus) error {

		if status == ClientStatusDisconnected {
			//slog.Warn("Server disconnected")
			return nil
		}

		sums[resp] += 1

		// continue the chain
		client := clients[resp]
		if time.Since(t0) < testLength {
			err = client.Send(resp)
			if err != nil {
				panic(fmt.Errorf("error sending request: %v", err))
			}
		} else {
			nReqResps.Add(sums[resp])
			wgWriters.Done()
			client.Close()
			clients[resp] = nil
		}

		return nil
	}

	for i := 0; i < nGoRoutines; i++ {
		client, err := NewClient[int32, int32](
			"localhost",
			server.Port(),
			nil,
			respHandler,
			codec.Writer,
			codec.Parser,
		)
		if err != nil {
			t.Fatalf("error creating client: %v", err)
		}
		clients[i] = client
	}

	defer func() {
		for i := 0; i < nGoRoutines; i++ {
			if clients[i] != nil {
				clients[i].Close()
			}
		}
	}()

	slog.Info("Starting chains")
	t0 = time.Now()
	lop.ForEach(lo.Range(nGoRoutines), func(i int, _ int) {
		client := clients[i]
		err = client.Send(int32(i))
		if err != nil {
			panic(fmt.Errorf("error sending request: %v", err))
		}
	})

	slog.Info("Waiting for chains to finish")
	wgWriters.Wait()

	slog.Info(fmt.Sprintf("Sent requests and received %v responses in %v", prettyInt3Digits(nReqResps.Load()), testLength))

}

func TestNewClient_SendAndRespondWithInts_1s_batched_performance_multiple_goroutines(t *testing.T) {
	snail_logging.ConfigureDefaultLogger("text", "info", false)

	slog.Info("TestNewServer_SendAndRespondWithInts")

	testLength := 1 * time.Second
	nGoRoutines := 512
	batchSize := 5 * 1024

	codec := snail_parser.NewInt32Codec()

	server, err := NewServer[int32, int32](
		func() ServerConnHandler[int32, int32] {
			return func(req int32, repFunc func(resp int32) error) error {
				if repFunc == nil {
					return nil
				}
				return repFunc(req)
			}
		},
		nil,
		codec.Parser,
		codec.Writer,
		&SnailServerOpts{Batcher: NewBatcherOpts(batchSize)},
	)

	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	defer server.Close()

	clients := make([]*SnailClient[int32, int32], nGoRoutines)

	sums := make([]int64, nGoRoutines)
	nReqResps := atomic.Int64{}

	wgWriters := sync.WaitGroup{}
	wgWriters.Add(nGoRoutines)
	respHandler := func(resp int32, status ClientStatus) error {

		if resp < 0 {
			resp = -resp - 1
			nReqResps.Add(sums[resp])
			wgWriters.Done()
			return nil
		}

		sums[resp] += 1

		return nil
	}

	for i := 0; i < nGoRoutines; i++ {
		client, err := NewClient[int32, int32](
			"localhost",
			server.Port(),
			nil,
			respHandler,
			codec.Writer,
			codec.Parser,
		)
		if err != nil {
			t.Fatalf("error creating client: %v", err)
		}
		clients[i] = client
	}

	defer func() {
		for i := 0; i < nGoRoutines; i++ {
			if clients[i] != nil {
				clients[i].Close()
			}
		}
	}()

	slog.Info("Creating batchers")
	batchers := make([]*snail_batcher.SnailBatcher[int32], nGoRoutines)
	lop.ForEach(lo.Range(nGoRoutines), func(i int, _ int) {
		client := clients[i]
		batchers[i] = snail_batcher.NewSnailBatcher[int32](
			1*time.Minute,
			batchSize,
			batchSize*2,
			func(values []int32) error {
				return client.SendBatchUnsafe(values)
			},
		)
	})

	withinTestWindow := atomic.Bool{}
	withinTestWindow.Store(true)
	go func() {
		time.Sleep(testLength)
		slog.Info("Test window is over")
		withinTestWindow.Store(false)
	}()

	slog.Info("Running senders")
	t0 := time.Now()
	lop.ForEach(lo.Range(nGoRoutines), func(i int, _ int) {

		batcher := batchers[i]

		for withinTestWindow.Load() {
			batcher.Add(int32(i))
		}

		batcher.Add(int32(-i - 1)) // Signal that this client is done
		batcher.Flush()
	})

	slog.Info("Senders are done")
	elapsedSend := time.Since(t0)

	slog.Info("Waiting to receive all responses")
	wgWriters.Wait()

	elapsedRecv := time.Since(t0)

	slog.Info(fmt.Sprintf("Sent %v requests in %v", prettyInt3Digits(nReqResps.Load()), elapsedSend))
	slog.Info(fmt.Sprintf("Received %v responses in %v", prettyInt3Digits(nReqResps.Load()), elapsedRecv))

	respRate := float64(nReqResps.Load()) / elapsedRecv.Seconds()
	sendRate := float64(nReqResps.Load()) / elapsedSend.Seconds()

	slog.Info(fmt.Sprintf("Response rate: %s items/sec", prettyInt3Digits(int64(respRate))))
	slog.Info(fmt.Sprintf("Send rate: %s items/sec", prettyInt3Digits(int64(sendRate))))

}

type stupidJsonStruct struct {
	Msg            string `json:"msg"`
	Bla            int    `json:"bla"`
	Foo            string `json:"foo"`
	Bar            int    `json:"bar"`
	GoRoutineIndex int    `json:"go_routine_index"`
	IsFinalMessage bool   `json:"is_final_message"`
}

func TestNewClient_SendAndRespondWithJson_1s_batched_performance_multiple_goroutines(t *testing.T) {
	snail_logging.ConfigureDefaultLogger("text", "info", false)

	slog.Info("TestNewClient_SendAndRespondWithJson_1s_batched_performance_multiple_goroutines")

	testLength := 1 * time.Second
	nGoRoutines := 200
	batchSize := 5 * 1024

	codec := snail_parser.NewJsonLinesCodec[stupidJsonStruct]()

	server, err := NewServer[stupidJsonStruct, stupidJsonStruct](
		func() ServerConnHandler[stupidJsonStruct, stupidJsonStruct] {
			return func(req stupidJsonStruct, repFunc func(resp stupidJsonStruct) error) error {
				if repFunc == nil {
					return nil
				}
				return repFunc(req)
			}
		},
		nil,
		codec.Parser,
		codec.Writer,
		&SnailServerOpts{Batcher: NewBatcherOpts(batchSize)},
	)

	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	defer server.Close()

	clients := make([]*SnailClient[stupidJsonStruct, stupidJsonStruct], nGoRoutines)

	sums := make([]int64, nGoRoutines)
	nReqResps := atomic.Int64{}

	wgWriters := sync.WaitGroup{}
	wgWriters.Add(nGoRoutines)
	respHandler := func(resp stupidJsonStruct, status ClientStatus) error {

		if resp.IsFinalMessage {
			nReqResps.Add(sums[resp.GoRoutineIndex])
			wgWriters.Done()
			return nil
		}

		sums[resp.GoRoutineIndex] += 1

		return nil
	}

	for i := 0; i < nGoRoutines; i++ {
		client, err := NewClient[stupidJsonStruct, stupidJsonStruct](
			"localhost",
			server.Port(),
			nil,
			respHandler,
			codec.Writer,
			codec.Parser,
		)
		if err != nil {
			t.Fatalf("error creating client: %v", err)
		}
		clients[i] = client
	}

	defer func() {
		for i := 0; i < nGoRoutines; i++ {
			if clients[i] != nil {
				clients[i].Close()
			}
		}
	}()

	slog.Info("Creating batchers")
	batchers := make([]*snail_batcher.SnailBatcher[stupidJsonStruct], nGoRoutines)
	lop.ForEach(lo.Range(nGoRoutines), func(i int, _ int) {
		client := clients[i]
		batchers[i] = snail_batcher.NewSnailBatcher[stupidJsonStruct](
			1*time.Minute,
			batchSize,
			batchSize*2,
			func(values []stupidJsonStruct) error {
				return client.SendBatchUnsafe(values)
			},
		)
	})

	withinTestWindow := atomic.Bool{}
	withinTestWindow.Store(true)
	go func() {
		time.Sleep(testLength)
		slog.Info("Test window is over")
		withinTestWindow.Store(false)
	}()

	slog.Info("Running senders")
	t0 := time.Now()
	lop.ForEach(lo.Range(nGoRoutines), func(i int, _ int) {

		batcher := batchers[i]

		for withinTestWindow.Load() {
			batcher.Add(stupidJsonStruct{
				Msg:            "Hello World",
				Bla:            123,
				Foo:            "321",
				Bar:            112233,
				GoRoutineIndex: i,
				IsFinalMessage: false,
			})
		}

		// Signal that this client is done
		batcher.Add(stupidJsonStruct{
			Msg:            "Test over",
			Bla:            123,
			Foo:            "321",
			Bar:            112233,
			GoRoutineIndex: i,
			IsFinalMessage: true,
		})

		batcher.Flush()
	})

	slog.Info("Senders are done")
	elapsedSend := time.Since(t0)

	slog.Info("Waiting to receive all responses")
	wgWriters.Wait()

	elapsedRecv := time.Since(t0)

	jsonAsBytes, err := json.Marshal(&stupidJsonStruct{
		Msg:            "Hello World",
		Bla:            123,
		Foo:            "321",
		Bar:            112233,
		GoRoutineIndex: 123,
		IsFinalMessage: false,
	})
	if err != nil {
		panic(fmt.Errorf("error marshalling json: %v", err))
	}
	msgSize := len(jsonAsBytes)
	totalDataBytes := nReqResps.Load() * int64(msgSize)

	slog.Info(fmt.Sprintf("Sent %v requests in %v", prettyInt3Digits(nReqResps.Load()), elapsedSend))
	slog.Info(fmt.Sprintf("Received %v responses in %v", prettyInt3Digits(nReqResps.Load()), elapsedRecv))
	slog.Info(fmt.Sprintf("Total data sent and received: %v bytes", prettyInt3Digits(totalDataBytes)))

	respRate := float64(nReqResps.Load()) / elapsedRecv.Seconds()
	sendRate := float64(nReqResps.Load()) / elapsedSend.Seconds()
	respRateBytesPerSec := float64(totalDataBytes) / elapsedRecv.Seconds() * 2 // *2 because we send and receive twice
	respRateBitsPerSec := respRateBytesPerSec * 8

	slog.Info(fmt.Sprintf("Response rate: %s items/sec", prettyInt3Digits(int64(respRate))))
	slog.Info(fmt.Sprintf("Response rate: %s bytes/sec", prettyInt3Digits(int64(respRateBytesPerSec))))
	slog.Info(fmt.Sprintf("Send rate: %s items/sec", prettyInt3Digits(int64(sendRate))))

	slog.Info(fmt.Sprintf("Total Bandwidth usage: %s bits/sec", prettyInt3Digits(int64(respRateBitsPerSec))))
}

var prettyPrinter = message.NewPrinter(language.English)

func prettyInt3Digits(n int64) string {
	return prettyPrinter.Sprintf("%d", n)
}
