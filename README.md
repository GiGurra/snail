Here's a what Anthropic claude 3.5 sonnet (and I) think about this project:

# Snail 🐌

A high-throughput networking library prototype focusing on TCP communication with request-response patterns.
The goal (which is mostly achieved) is to be capable of sending and receiving very small request-responses
at a rate maxing out any reasonably modern network interface (1-10 GBit/s).

While this library and README is focused on TCP communication, the implementation is modular and its usefulness is not
specific to TCP - I just chose it as the first use-case. The library is designed to be cherry-picked from for other
situations. For example, it would probably also be useful for creating fast solutions for writing to log files,
batching requests to other system devices, or other similar scenarios.

## Initial Motivation

Web servers, microservices and many system architectures are built on top of the request-response pattern.
However, virtually all of them are also built on top of tcp (which is also the assumed transport layer for most cloud
based environments such as Kubernetes and its networking), and naive request-response patterns built on top of tcp is
an incredibly inefficient use of available network capacity. While request-response designs give the
user/developer a sense of the operations being lightweight RPCs similar to regular function calls, tcp communication is
optimized for continuous streams of data and maximizing bandwidth/throughput and managing such traffic. If you just
write a simple naively implemented RPC over tcp, you would be lucky to achieve more than 10-20k requests/s per
thread/goroutine.

On the other hand, the request-response pattern is easy to reason about, easy to implement, easy to guarantee
transactions and data/state consistency with across distributed systems. It also is the basis for a vast amount of
existing standard tools and systems that would be difficult to live without - and sometimes/often it simply maps 1:1 to
the real life problems. Asynchronous message passing without explicit dedicated responses (actors) suffers from the same
fundamental problem with TCP-like transports. This project focuses on the request-response pattern in the context of
microservices, web servers etc, but the same efficiency gains presented here can be achieved in the distributed actor
model as well.

This library aims to make traditional request-response situations more efficient, without modifying the
request-response programming interface used by developers. The implementation is designed to be modular and to be
cherry-picked from (does not 100% assume TCP or a request response pattern, but to make use of the
full benefits of all components in this library, at least for now, TCP and request-response is assumed).

In other words, your programming interface is expected to look something like this, and will stay like this after
adopting `snail`:

```go
func someOperation(data Data) SomeResult {
// ...
}
```

## Performance Goal

I want to be able to program against a request-response interface, with somewhat arbitrarily sized requests and
responses, while still achieving throughput maxing out my/my services'/my pods' network interface bandwidth. I don't
want to have to worry about the network layer's max request rates (within reason, i.e. as long as the requests are
independent).

## High level design/solution?

How can this be addressed? Have you ever played a game such as Transport Tycoon? In that game you try to build a
transportation company - optimizing transportation of goods and people for maximum profit. You build tracks, train
stations and trains. Then you send your trains on different circular patterns to pick up and deliver goods. You
configure your trains to wait at stations based on different criteria. Sometimes it is useful to wait for a full load of
goods, while other times it is useful to wait for a certain amount of time. Sometimes we just want to send data
immediately. This is the same concept we are using here. We are building train stations for our data, and we are
configuring our trains to wait for a certain amount of time or a certain amount of data before they leave the station.
This is the main mechanism we are using in this library.

So, when you make a request using `snail` (or similar implementation), the request is not sent immediately - but you
still just see a regular function call and response, like with any rpc library. Under the hood though, `snail` puts your
request in a queue/batch. When the batch reaches a certain size, or a certain amount of time has passed (whichever comes
first according to your configuration), the entire batch of requests is sent to the server.
Similarly, on the server side, different requests can be processed in parallel, and finish at different times. When a
response is ready, it is not sent immediately. Instead, it is put in a suitable batch on the server dedicated to the
client that sent the request. Different responses may join different response batches - the order of operations can be
random and `snail` won't force you to respond in the same order the operations were requested in (although you can - if
you want to). In the exact same way as on the client side, when the batch reaches a certain size, or a certain
amount of time has passed (whichever comes first according to your configuration) - or some other criteria you create
has been met (e.g. you explicitly call `SnailBatcher.Flush()`), a batch of responses is sent to the client. (Btw, the
`SnailBatcher[ElemT]` is a generic batcher. Not connected to networking, tcp, or anything like that. It just consumes
individual elements and outputs batches on a callback function).

It's up to you to choose the batch size and the time limit for the batch to be sent, according to your needs. This is
similar to the old Nagle algorithm - but you can configure and extend it to match the needs of the application and
larger system architecture.

### Lessons learned

While building this library, many different implementations and designs were tried. The generic batcher itself is
probably on its 10th implementation so far :). It's still mostly just a prototype, but a few of the things have been
learned:

* Implementing batching/the train station concept for request-responses is basically a challenge of:
    * How to pick the right batch size, how to store batches and manage memory allocation, and how to auto time
      batches.
    * How to efficiently coordinate the input from multiple goroutines and have them take their seats in a single
      batch/train. (fan-in)
    * How to handle the return trip of batches/trains, and how the different responses are most efficiently delivered
      back to the original goroutines/sources of the requests. This can be in totally different order than the requests
      were made. (fan-out)
* It was found that, for fan-in (and in simplified benchmarks just doing atomic addition across many goroutines), the
  fastest coordination mechanism in a highly contended environment is:
    * Mutex spinning until the batch is full, and then regular mutex locking and waiting until a new batch is
      available for writing
    * About half the speed:
        * atomics with non-blocking design
        * regular mutexes
    * About 1/50th the speed:
        * channels (slowest)
* Go channels are surprisingly slow for high-contention scenarios
    * You can expect low single digit millions of elements per second on a highly contended fan-in channel, while
      reaching maybe 30-40 million elements per second on a non-contended channel. Our situation is by design highly
      contended, and we aim for 100s of millions of elements per second - therefor a pure channel based fan-in is not an
      option.
    * However: Channels are fantastic for efficiently transferring prepared batches between different goroutines.
* The Go standard library mutex implementation performs quite differently on different platforms and in different
  contention levels. For example, on MacOS using Apple Silicon, if is very fast if not contended - but if contended, it
  is incredibly very slow between 2-8 concurrent goroutines - but actually becomes a little faster at increased
  goroutine counts. The implementation on x86_64 Linux on the other hand has a more smooth performance curve, gradually
  going down from 150 to 20-25 million operations/second on my computer (see specs below) - faster than MacOS up to
  around 10-20 concurrent goroutines, after which the MacOS implementation takes the lead (the goland profiler suggests
  the macos implementation gets stuck in `usleep` operations too long, and you need to have more goroutines to counter
  this effect).
* The fan-in problem combined with batching is difficult to do efficiently. Neither channels nor mutexes alone solves
  the problem entirely on their own.
* Writing non-blocking/atomics based algorithms (e.g. for fan-in) is very difficult to get right, and the performance
  can often be worse than regular mutexes.
* Memory allocation is expensive in Go (and in most non-moving/non-generational systems, or so I've heard :S)
  when you try to shuffle around 10-20 GB/s (see below why this number matters). On my linux machine, you will be
  spending a very significant CPU time on memory allocation if you are not careful. So if you want to shuffle data at
  that rate (which we want to, see below), you'll need to either avoid allocations entirely, or use a custom memory
  allocator/object pooling.
* Atomics are cool, but not always faster than mutexes.
    * It's very platform dependent
* Be prepared to experiment with custom mutex alternatives.
    * The internal `snail` batcher uses a combination of spin locking and regular mutex locking
* Be prepared to write custom memory allocators.
* Be prepared to write custom data serializers/deserializers
* Be prepared to spend a lot of time in the profiler. The profiler is your friend.
* Back pressure the entire communication chain if you don't want to run out of memory, or unexpected system overflows if
  a node in the chain unexpectedly goes down. This is in practice guaranteed with `snail` by using blocking io and
  queueing operations and appropriately configured buffer sizes, made easier by the lightweight nature of Go's
  goroutines allowing for simple synchronous programming.

## Features

- Performant TCP client/server implementation
- Request-response pattern support
- Efficient batching mechanism
- Support for pluggable encoding formats (JSON, custom binary protocols)
    - You just plug in a parser and a writer for your custom data type. `snail` servers and clients are type
      parameterized with the `Request` and `Response` types for this purpose, and for the purpose of providing an easy
      to use and type safe programming interface.
- Configurable optimization for latency or throughput, or something in between
    - Auto-flush size
    - Auto-flush time
    - Read/write buffer sizes
    - Option to manually flush

## Installation

```bash
go get github.com/GiGurra/snail
```

## Quick Start

Below is an example using a very basic json messaging format. This is mostly for educational purposes, as it uses
golang's built-in json library which is not the most efficient (aside from JSON itself not exactly being the most
efficient format). See performance numbers further down for more information.

Remember that this is a prototype, the function signatures may change.

<details>

<summary>example code</summary>

```go
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
				if repFunc == nil {
					slog.Warn("Disconnected")
					return nil
				}
				// Here we reply immediately, but you can also reply later
				// and store the repFunc for later use
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
				// or maybe implement auto reconnect features
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

```

</details>

## Preliminary Benchmarks

Measured on

- 7950x3d CPU
- 64GB of 6000MT/s dual channel RAM
- Ubuntu 24.04
- loopback tcp connections

Reference Numbers:

- Reference max bandwidth achieved on loopback:
    - 800+ GBit/s using `iperf3` test tool
    - 400+ GBit/s using go `net` package

Achieved performance with `snail`:

- Request rate with 4 byte requests/responses: 300-350 million request-responses/s
    - Each request and response is just a single int32, encoded to big endian
    - This equates to about 20 GBit/s throughput
        - half of which is the data sent to the server
        - half of which is the data sent back to the client
- Request rate with 276 byte requests/responses: 30 million request-responses/s
    - Each request and response is a custom struct with
        - 3x encoded/decoded integer fields (1x int32 + 2x int64)
        - 1x 256 byte array at the end
        - custom encoder/parser
    - This equates to about 130 Gbit/s throughput
        - half of which is the data sent to the server
        - half of which is the data sent back to the client
    - Each request is fully parsed and encoded on both the client and server side
- Request rate with http1.1 using `h2load` as load generator: 20-25 million request-responses/s
    - We parse very minimal parts of the incoming http request for testing purposes
    - http 1.1 pipelining is enabled
        - a comparable result should be achievable with http2 multiplexing
    - This equates to about 30 Gbit/s throughput (according to `h2load`)
        - half of which is the data sent to the server
        - half of which is the data sent back to the client
- Request rate with http1.1 using a [custom built http1.1 load testing tool](cmd/cmd_load/cmd_load_h1/cmd_load_h1.go):
  45 million request-responses/s
    - Total traffic is about 60-65 Gbit/s (total of send+receive)
    - Our own custom tool cheats a LOT :).
- Request rate with json payload: 5 million request-responses/s
    - Almost all time spent in go std lib json marshalling/unmarshalling (>80%)
    - A considerable amount of time spent in memory allocation/malloc
    - We should try a more efficient json parsing/encoding library

What does `request-responses` mean?

- A request is exactly one go function call with the request type as argument
- A response is exactly one go function callback with the response type as argument

Type used in the json tests:

```go
package yourpackage

type stupidJsonStruct struct {
	Msg            string `json:"msg"`
	Bla            int    `json:"bla"`
	Foo            string `json:"foo"`
	Bar            int    `json:"bar"`
	GoRoutineIndex int    `json:"go_routine_index"`
	IsFinalMessage bool   `json:"is_final_message"`
}

```

### Experimental future features used in testing and benchmarking

- Memory pooling for reduced GC pressure
- Support for custom allocators

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Readme  Authors

- GiGurra
- Anthropic Claude 3.5 sonnet
- Github Copilot

## Code Authors

- GiGurra
- Github Copilot
- Anthropic Claude 3.5 sonnet