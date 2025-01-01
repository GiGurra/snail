Here's a what Anthropic claude 3.5 sonnet thinks about this project:

# Snail 🐌

A high-throughput networking library prototype focusing on TCP communication with request-response patterns.

While this library and README is focused on TCP communication, the implementation is modular and its usefulness is not
specific to TCP - I just chose it as the first use-case. The library is designed to be cherry-picked from for other
situations. For example, it would probably also be useful for creating fast solutions for writing to log files,
batching requests to other system devices, or other similar scenarios.

## Initial Motivation

Web servers, microservices and many system architectures are built on top of the request-response pattern.
However, virtually all of them are also built on top of tcp (which is also the assumed transport layer for most cloud
based environments such as Kubernetes and its networking), and the tcp stack implemented in modern operating
systems is incredibly inefficient for the request-response pattern. While request-response designs give the
user/developer a sense of the operations being lightweight RPCs similar to regular function calls, tcp communication is
optimized for continuous streams of data and maximizing bandwidth/throughput and managing such traffic. If you just
write a simple naively implemented RPC over tcp, you would be lucky to achieve more than 10-20k requests/s per
thread/goroutine. Regular function calls can easily reach the 100s of millions of calls per second.

On the other hand, the request-response pattern is easy to reason about, easy to implement, easy to guarantee
transactions and data/state consistency with across distributed systems. It also is the basis for a vast amount of
existing standard tools and systems that would be difficult to live without - and sometimes/often it simply maps 1:1 to
the real life problems. Asynchronous message passing without explicit dedicated responses (actors) suffers from the same
fundamental problem with TCP-like transports. This project focuses on the request-response pattern in the context of
microservices, web servers etc, but the same efficiency gains presented here can be achieved in the distributed actor
model as well.

This library aims to make traditional request-response situations more efficient, without modifying the
request-response programming interface used by developers, by serializing concurrent requests from high number of
parallel operations into a single data stream - both client and server side. The implementation is designed to be
modular and to be cherry-picked from (does not 100% assume TCP or a request response pattern, but to make use of the
full benefits of all components in this library, at least for now, TCP and request-response is assumed).

In other words, your programming interface is expected to look something like this, and will stay like this after
adopting `snail`:

```go
func someOperation(data Data) SomeResult {
// ...
}
```

## High level design/solution?

How can this be addressed? Have you ever played a game such as Transport Tycoon? In that game you try to build a
transportation company - optimizing transportation of goods and people for maximum profit. You build tracks, train
stations and trains. Then you send your trains on different circular patterns to pick up and deliver goods. You
configure your trains to wait at stations based on different criteria. Sometimes it is useful to wait for a full load of
goods, while other times it is useful to wait for a certain amount of time. Sometimes we just want to send data
immediately. This is the same concept we are using here. We are building train stations for our data, and we are
configuring our trains to wait for a certain amount of time or a certain amount of data before they leave the station.
This is the main mechanism we are using in this library.

So, when you for example create a http client with `snail`, and make a request through it, the request is not sent
immediately - but you still just see a regular function call and response, like with any http library. Under the hood
though, `snail` puts your request in a queue/batch. When the batch reaches a certain size, or a certain amount of time
has passed (whichever comes first according to your configuration), the entire batch of requests is sent to the server.
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

* If you aim to create a system capable of achieving for example 100 million elements/messages per second on a global
  level (=through the chain of steps/components in entire system including TCP transmissions, service->service), which
  is roughly the level we aim for, each individual step in the chain must be significantly faster than 100 million
  elements/messages per second.
* Implementing batching/the train station concept for request-responses is basically a challenge of:
    * Batching
        * How to pick the right batch size, how to store batches and manage memory allocation, and how to auto time
          batches.
    * Fan-in
        * How to efficiently coordinate the input from multiple goroutines and have them take their seats in a single
          batch/train.
    * Fan-out
        * How to handle the return trip of trans, and how the different responses are most efficiently delivered back to
          the original goroutines/sources of the requests. This can be in totally different order than the requests were
          made.
* Channels are surprisingly slow for high-contention scenarios
    * You can expect single digit millions of elements per second on a highly contended fan-in channel, while reaching
      maybe 30-40 million elements per second on a non-contended channel. Our situation is by design highly contended,
      and we aim for 100s of millions of elements per second - therefor a pure channel based fan-in is not an option (
      This was the design I tried at first and where I discovered the performance limitations of channels :) ).
    * If your application does absolutely nothing else but shuffle elements of data between channels, fully utilizing
      all cores (tests on 7950X3D below) pushing to dedicated non-contended channels (NOT our case), you could perhaps
      hit 300-400 million elements per second. Again, that's if your application did absolutely nothing else and had no
      other purpose than moving elements around without and logic, without fan in, without fan out, without computations
      or transmissions elsewhere :). But we do a lot more, and have a lot of hand-over steps both on the client and
      server side. A single hand over cannot consume all of our available CPU or kernel resources, or we will not be
      able to achieve the desired global throughput.
    * For efficient fan-out and fan-in when aiming for throughput on the order of 100s of millions of elements per
      second for your system as a whole/globally, channels are just simply too slow, by 1-2 orders of magnitude.
* The Go standard library mutex implementation performs wildly different on different platforms and in different
  contention levels. For example, on MacOS using Apple Silicon, if is very fast if not contended - but if contended, it
  is incredibly very slow between 2-8 concurrent goroutines (only achieving single digit million of operations per
  second, compared to 100-150 m/s at zero contention). Surprisingly, above 2-8 it actually becomes faster (not just
  more efficient), shooting up to 40+ before leveling off around 30 million operations/s (tested on m4 pro). The
  implementation on Linux on the other hand has a more smooth performance curve, gradually going down to 20-25 million
  operations/second on my computer (see specs below) - faster than MacOS up to around 10-20 concurrent goroutines, after
  which the MacOS implementation takes the lead (the goland profiler suggests the macos implementation gets stuck in
  usleep operations too long, and you need to have more goroutines to counter this effect).
    * Either way, just having a mutex is not fast enough on either system. it is faster than channels by 2-5x, but this
      is not enough for our purposes.
* The fan-in problem combined with batching is difficult to do efficiently. Neither channels or mutexes alone solves the
  problem.
* Memory allocation is grossly expensive in Go (and in non-moving/non-generational systems). At about 10-20 GB/s, you
  will be spending all of your time in memory allocation. So if you want to shuffle data at that rate (which we want to,
  see below), you'll need to either avoid allocations entirely, or use a custom memory allocator/object pooling.
* Circular buffers are awesome. Double/triple buffers are awesome. Atomics are awesome :).
    * This is how `snail` achieves its throughput in batching.
* Be prepared to experiment with custom mutex implementations.
* Be prepared to write custom memory allocators.
* Be prepared to write custom data serializers/deserializers
* Be prepared to write custom data structures.
* Be prepared to spend a lot of time in the profiler. The profiler is your friend.

## Features

- High-performance TCP client/server implementation
- Request-response pattern support
- Efficient batching mechanism
- Support for different encoding formats (JSON, custom binary protocols)
- Buffer management with memory pooling options
- Configurable optimization for latency or throughput

## Installation

```bash
go get github.com/GiGurra/snail
```

## Quick Start

### Shared

```go
package yourpackage

type requestStruct struct {
	Msg string
}

type responseStruct struct {
	Msg string
}

func main() {
	reqCodec := snail_parser.NewJsonLinesCodec[requestStruct]()
	respCodec := snail_parser.NewJsonLinesCodec[responseStruct]()
	// ... server/client code here
}
```

### Simple JSON Server

```go
package yourpackage

func main() {
	server, err := NewServer[requestStruct, responseStruct](
		func() ServerConnHandler[requestStruct, responseStruct] {
			return func(req requestStruct, repFunc func(resp responseStruct) error) error {
				return repFunc(responseStruct{Msg: "Hello from server"})
			}
		},
		nil,
		reqCodec.Parser,
		respCodec.Writer,
		nil,
	)
}

```

### Simple JSON Client

```go
package yourpackage

func main() {
	client, err := NewClient[requestStruct, responseStruct](
		"localhost",
		server.Port(),
		nil,
		func(resp responseStruct, status ClientStatus) error {
			fmt.Printf("Received response: %v\n", resp)
			return nil
		},
		reqCodec.Writer,
		respCodec.Parser,
	)

	// Send a request
	client.Send(requestStruct{Msg: "Hello"})
}

```

## Performance Features

- Efficient batching and fan-in/fan-out pattern implementation
    - Flushes when batch size or time limit is reached, whichever comes first
        - Basically a controllable Nagle-ish algorithm (chose your own time and size parameters!)
    - Uses a combination of atomics, locks and channels.
    - Regular channels are insufficient for high-throughput scenarios when message size is small,
      and the custom system is about 10x faster on average.
    - Fan-in is especially tricky, but is solved using an n-buffer solution (default=triple buffering),
      inspired by game programming.
- Configurable for both latency and throughput use cases
    - Configurable read/write buffer sizes
    - Configurable flush window timing
- Can be configured for close to zero allocations

## Preliminary Benchmarks

Measured on

- 7950x3d CPU
- 64GB of 6000MT/s dual channel RAM
- loopback tcp connections
- WSL2 Ubuntu 22.04 on Windows 11

Numbers:

- Reference max bandwidth achieved on loopback: 240-250 GBit/s (using `snail`)
    - achieved by sending larger byte chunks as messages
    - `hperf3` achieved only about 135 GBit/s, but I'm probably using it wrong
- Request rate with 4 byte requests/responses: 300-350 million request-responses/s
    - Each request and response is just a single int32
    - This equates to about 20 GBit/s throughput
        - half of which is the data sent to the server
        - half of which is the data sent back to the client
- Request rate with 276 byte requests/responses: 25 million request-responses/s
    - Each request and response is a custom struct with
        - 3x encoded/decoded integer fields (1x int32 + 2x int64)
        - 1x 256 byte array at the end
        - custom encoder/parser
    - This equates to about 110 Gbit/s throughput
        - half of which is the data sent to the server
        - half of which is the data sent back to the client
- Request rate with http1.1 using `h2load` as load generator: 20-25 million request-responses/s
    - We parse very minimal parts of the incoming http request for testing purposes
    - http 1.1 pipelining is enabled
        - a comparable result should be achievable with http2 multiplexing
    - We should investigate `h2load` to se if it is the bottleneck
    - This equates to about 40 Gbit/s throughput (according to `h2load`)
        - half of which is the data sent to the server
        - half of which is the data sent back to the client
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

### Experimental features used in testing and benchmarking

- Memory pooling for reduced GC pressure
- Support for custom allocators

## Configuration

### Batching Options

```go
package snail_batcher

type BatcherOpts struct {
	WindowSize time.Duration
	BatchSize  int
	QueueSize  int
}

```

## Performance Tips

1. Use batching for high-throughput scenarios
2. Configure appropriate buffer sizes
3. Choose the right optimization mode (latency vs throughput)
4. Consider using custom memory allocators for reduced GC pressure if needed

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Readme  Authors

- Anthropic Claude 3.5

## Code Authors

- GiGurra
- Github Copilot
- Anthropic Claude 3.5 sonnet