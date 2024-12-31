package snail_tcp_reqrep

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_batcher"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_parser"
	"github.com/GiGurra/snail/pkg/snail_tcp"
	"github.com/samber/lo"
	"net"
	"sync"
	"time"
)

// ServerConnHandler is the custom handler for a server connection. If the socket is closed, nil, nil is called
type ServerConnHandler[Req any, Resp any] func(
	req Req,
	repFunc func(resp Resp) error,
) error

type SnailServer[Req any, Resp any] struct {
	underlying     *snail_tcp.SnailServer
	newHandlerFunc func() ServerConnHandler[Req, Resp]
	parseFunc      snail_parser.ParseFunc[Req]
	writeFunc      snail_parser.WriteFunc[Resp]
	opts           SnailServerOpts[Req, Resp]
}

type BatcherOpts struct {
	WindowSize time.Duration
	BatchSize  int
	QueueSize  int
}

func NewBatcherOpts(batchSize int) BatcherOpts {
	return BatcherOpts{
		BatchSize: batchSize,
		QueueSize: batchSize * 2, //default to triple buffering
	}.WithDefaults()
}

func (b BatcherOpts) WithDefaults() BatcherOpts {
	if b.IsEnabled() {
		if b.WindowSize == 0 {
			b.WindowSize = 25 * time.Millisecond
		}
		if b.QueueSize == 0 {
			b.QueueSize = 2 * b.BatchSize
		}
	}
	return b
}

func (b BatcherOpts) IsEnabled() bool {
	return b.BatchSize > 0
}

type SnailServerOpts[Req any, Resp any] struct {
	Batcher      BatcherOpts // will be created per conn by the server implementation
	PerConnCodec func() PerConnCodec[Req, Resp]
}

func (s SnailServerOpts[Req, Resp]) WithDefaults() SnailServerOpts[Req, Resp] {
	s.Batcher = s.Batcher.WithDefaults()
	return s
}

func (s SnailServerOpts[Req, Resp]) WidthBatching(opts BatcherOpts) SnailServerOpts[Req, Resp] {
	s.Batcher = opts
	return s
}

func (s SnailServerOpts[Req, Resp]) WithPerConnCodec(codecFunc func() PerConnCodec[Req, Resp]) SnailServerOpts[Req, Resp] {
	s.PerConnCodec = codecFunc
	return s
}

func (s SnailServerOpts[Req, Resp]) validate() {
	if s.Batcher.IsEnabled() {
		if s.Batcher.BatchSize <= 0 {
			panic(fmt.Sprintf("BatchSize must be > 0, got %d", s.Batcher.BatchSize))
		}
		if s.Batcher.WindowSize <= 0 {
			panic(fmt.Sprintf("WindowSize must be > 0, got %d", s.Batcher.WindowSize))
		}
		if s.Batcher.QueueSize <= 0 {
			panic(fmt.Sprintf("QueueSize must be > 0, got %d", s.Batcher.QueueSize))
		}
		// queue size must be a multiple of batch size
		if s.Batcher.QueueSize%s.Batcher.BatchSize != 0 {
			panic(fmt.Sprintf("QueueSize must be a multiple of BatchSize, got %d", s.Batcher.QueueSize))
		}
	}
}

type PerConnCodec[Req any, Resp any] struct {
	ParseFunc snail_parser.ParseFunc[Req]
	WriteFunc snail_parser.WriteFunc[Resp]
}

func NewServer[Req any, Resp any](
	newHandlerFunc func() ServerConnHandler[Req, Resp],
	tcpOpts *snail_tcp.SnailServerOpts,
	parseFunc snail_parser.ParseFunc[Req],
	writeFunc snail_parser.WriteFunc[Resp],
	opts *SnailServerOpts[Req, Resp],
) (*SnailServer[Req, Resp], error) {

	if opts == nil {
		opts = &SnailServerOpts[Req, Resp]{}
	}
	opts = lo.ToPtr(opts.WithDefaults())
	opts.validate()

	if parseFunc == nil || writeFunc == nil {
		if opts.PerConnCodec == nil {
			return nil, fmt.Errorf("parseFunc and writeFunc must be provided if opts.PerConnCodec is nil")
		}
	}

	newTcpHandlerFunc := func(conn net.Conn) snail_tcp.ServerConnHandler {
		ownParseFunc := parseFunc
		ownWriteFunc := writeFunc
		if opts.PerConnCodec != nil {
			codec := opts.PerConnCodec()
			ownParseFunc = codec.ParseFunc
			ownWriteFunc = codec.WriteFunc
		}
		return newTcpServerConnHandler[Req, Resp](newHandlerFunc, ownParseFunc, ownWriteFunc, opts.Batcher, conn)
	}

	underlying, err := snail_tcp.NewServer(newTcpHandlerFunc, tcpOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create underlying server: %w", err)
	}

	return &SnailServer[Req, Resp]{
		underlying:     underlying,
		newHandlerFunc: newHandlerFunc,
		parseFunc:      parseFunc,
		writeFunc:      writeFunc,
		opts:           *opts,
	}, nil
}

func (s *SnailServer[Req, Resp]) Underlying() *snail_tcp.SnailServer {
	return s.underlying
}

func (s *SnailServer[Req, Resp]) Port() int {
	return s.underlying.Port()
}

func (s *SnailServer[Req, Resp]) Close() {
	s.underlying.Close()
}

func newTcpServerConnHandler[Req any, Resp any](
	userHandlerFunc func() ServerConnHandler[Req, Resp],
	parseFunc snail_parser.ParseFunc[Req],
	writeFunc snail_parser.WriteFunc[Resp],
	batcherOpts BatcherOpts,
	conn net.Conn,
) snail_tcp.ServerConnHandler {

	var batcher *snail_batcher.SnailBatcher[Resp]
	if batcherOpts.IsEnabled() {
		writeBuffer := snail_buffer.New(snail_buffer.BigEndian, 64*1024)
		batcher = snail_batcher.NewSnailBatcher[Resp](
			batcherOpts.BatchSize,
			batcherOpts.QueueSize,
			false,
			batcherOpts.WindowSize,
			func(resps []Resp) error {

				// We don't need a mutex to protect the writeBuffer here, since
				// the batcher will only call this function from a single thread.
				defer writeBuffer.Reset()

				// Prepare the response
				for _, resp := range resps {
					if err := writeFunc(writeBuffer, resp); err != nil {
						return fmt.Errorf("failed to write response: %w", err)
					}
				}

				// Write the response
				err := snail_tcp.SendAll(conn, writeBuffer.Underlying())
				if err != nil {
					return fmt.Errorf("failed to write response: %w", err)
				}

				return nil
			},
		)
	}

	var writeRespFunc func(resp Resp) error

	if batcher != nil {
		writeRespFunc = func(resp Resp) error {
			batcher.Add(resp) // TODO: Propagate errors?
			return nil
		}
	} else {

		// Non-batched mode

		writeBuffer := snail_buffer.New(snail_buffer.BigEndian, 64*1024)
		writeMutex := sync.Mutex{}
		writeRespFunc = func(resp Resp) error {

			// A mutex is likely to be faster than a channel here, since we are
			// dealing with n multiplexed requests over a single connection.
			// They are likely to be in the range of 1-1000.
			// We have to lock it here, because we don't know when the callback
			// will be called. It could be asynchronous, and much later, when a different
			// request is being processed.

			writeMutex.Lock()
			defer writeMutex.Unlock()
			defer writeBuffer.Reset()

			// Prepare the response
			if err := writeFunc(writeBuffer, resp); err != nil {
				return fmt.Errorf("failed to write response: %w", err)
			}

			// Write the response
			err := snail_tcp.SendAll(conn, writeBuffer.Underlying())
			if err != nil {
				return fmt.Errorf("failed to write response: %w", err)
			}

			return nil
		}

	}

	userHandler := userHandlerFunc()
	tcpHandler := func(readBuffer *snail_buffer.Buffer) error {

		if readBuffer == nil {
			var zero Req
			err := userHandler(zero, nil)
			if batcher != nil {
				batcher.Close()
			}
			return err
		}

		reqs, err := snail_parser.ParseAll[Req](readBuffer, parseFunc)
		if err != nil {
			return fmt.Errorf("failed to parse requests: %w", err)
		}

		for _, req := range reqs {
			if err := userHandler(req, writeRespFunc); err != nil {
				return fmt.Errorf("failed to handle request: %w", err)
			}
		}

		return nil
	}

	return tcpHandler
}
