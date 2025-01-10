package main

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_batcher"
	"github.com/GiGurra/snail/pkg/snail_parser"
	"github.com/GiGurra/snail/pkg/snail_tcp"
	"github.com/GiGurra/snail/pkg/snail_tcp_reqrep"
	"log/slog"
	"time"
)

type requestStruct struct {
	Msg string `json:"msg"`
}

type responseStruct struct {
	Msg string `json:"msg"`
}

var reqCodec = snail_parser.NewJsonLinesCodec[requestStruct]()
var respCodec = snail_parser.NewJsonLinesCodec[responseStruct]()

func main() {

	server, err := snail_tcp_reqrep.NewServer[requestStruct, responseStruct](
		func() snail_tcp_reqrep.ServerConnHandler[requestStruct, responseStruct] {
			return func(req requestStruct, repFunc func(resp responseStruct) error) error {
				return repFunc(responseStruct{Msg: "Reply from server, to: " + req.Msg})
			}
		},
		&snail_tcp.SnailServerOpts{}, // tcpOpts, like port, read/write buffer sizes, regular nagle on/off, tcp window sizes etc
		reqCodec.Parser,              // just a function
		respCodec.Writer,             // just a function
		&snail_tcp_reqrep.SnailServerOpts[requestStruct, responseStruct]{
			Batcher: snail_tcp_reqrep.BatcherOpts{
				WindowSize: 25 * time.Millisecond, // auto flush timer if buffer isn't filled up in time
				BatchSize:  100,                   // max number of requests to batch up before sending
				QueueSize:  200,                   // max number of requests to queue up after the batch before blocking
			},
			PerConnCodec: nil, // for per-connection stateful codecs
		},
	)
	if err != nil {
		panic(fmt.Sprintf("error creating server: %v", err))
	}
	defer server.Close()

	port := server.Port()

	fmt.Printf("Server running on port %d\n", port)

	client, err := snail_tcp_reqrep.NewClient[requestStruct, responseStruct](
		"localhost",
		port,
		&snail_tcp.SnailClientOpts{}, // tcpOpts, like port, read/write buffer sizes, regular nagle on/off, tcp window sizes etc
		func(resp responseStruct, status snail_tcp_reqrep.ClientStatus) error {
			if status == snail_tcp_reqrep.ClientStatusDisconnected {
				slog.Warn("Disconnected")
				return nil
			}
			fmt.Printf("Received response: %v\n", resp)
			// Fan-out is optional, but should be added here if you want to have an rpc like interface
			return nil
		},
		reqCodec.Writer,  // just a function
		respCodec.Parser, // just a function
	)
	if err != nil {
		panic(fmt.Sprintf("error creating client: %v", err))
	}
	defer client.Close()

	// Send un-batched requests
	err = client.Send(requestStruct{Msg: "Hello"})
	if err != nil {
		panic(fmt.Sprintf("error sending request: %v", err))
	}

	// batching
	batcher := snail_batcher.NewSnailBatcher[requestStruct](
		100,                 // max number of requests to batch up before sending
		200,                 // max number of requests to queue up after the batch before blocking
		25*time.Millisecond, // auto flush timer if buffer isn't filled up in time
		func(batch []requestStruct) error {
			err := client.SendBatch(batch)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to send batch: %v", err))
				// TODO: handle error, signal back to your application
				// or return fmt.Errorf("failed to send batch: %v", err) to the batcher (will currently just log it)
			}
			return nil
		},
	)
	defer batcher.Close()

	// Send batched requests
	for i := 0; i < 1000; i++ {
		batcher.Add(requestStruct{Msg: fmt.Sprintf("Hello %d", i)})
	}

	// Prints
	// Received response: {Reply from server, to: Hello}
	// Received response: {Reply from server, to: Hello 0}
	// Received response: {Reply from server, to: Hello 1}
	// Received response: {Reply from server, to: Hello 2}
	// Received response: {Reply from server, to: Hello 3}
	// Received response: {Reply from server, to: Hello 4}
	// Received response: {Reply from server, to: Hello 5}
	// ...

	// Let the operations run, just sleep, or maybe use a wait group ;)
	time.Sleep(1 * time.Second)

}
