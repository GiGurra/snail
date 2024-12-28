package snail_tcp_reqrep

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"github.com/GiGurra/snail/pkg/snail_parser"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
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

	slog.Info("Sent and received", slog.Int64("nReqResps", nReqResps.Load()))

}
