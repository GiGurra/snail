package snail_tcp_reqrep

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_parser"
	"github.com/GiGurra/snail/pkg/snail_tcp"
	"io"
	"sync"
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
}

func NewServer[Req any, Resp any](
	newHandlerFunc func() ServerConnHandler[Req, Resp],
	tcpOpts *snail_tcp.SnailServerOpts,
	parseFunc snail_parser.ParseFunc[Req],
	writeFunc snail_parser.WriteFunc[Resp],
) (*SnailServer[Req, Resp], error) {

	newTcpHandlerFunc := func() snail_tcp.ServerConnHandler {
		return newTcpServerConnHandler[Req, Resp](newHandlerFunc, parseFunc, writeFunc)
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
	newHandlerFunc func() ServerConnHandler[Req, Resp],
	parseFunc snail_parser.ParseFunc[Req],
	writeFunc snail_parser.WriteFunc[Resp],
) snail_tcp.ServerConnHandler {

	handler := newHandlerFunc()
	writeBuffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
	writeMutex := sync.Mutex{}

	tcpHandler := func(readBuffer *snail_buffer.Buffer, writer io.Writer) error {

		if readBuffer == nil || writer == nil {
			var zero Req
			return handler(zero, nil)
		}

		reqs, err := snail_parser.ParseAll[Req](readBuffer, parseFunc)
		if err != nil {
			return fmt.Errorf("failed to parse requests: %w", err)
		}

		// A mutex is likely to be faster than a channel here, since we are
		// dealing with n multiplexed requests over a single connection.
		// They are likely to be in the range of 1-1000.
		writeRespFunc := func(resp Resp) error {
			writeMutex.Lock()
			defer writeMutex.Unlock()

			if err := writeFunc(writeBuffer, resp); err != nil {
				return fmt.Errorf("failed to write response: %w", err)
			}

			// Write the response
			bytes := writeBuffer.Underlying()
			nTotal := len(bytes)
			nWritten := 0
			for nWritten < nTotal {
				n, err := writer.Write(bytes[nWritten:])
				if err != nil {
					return fmt.Errorf("failed to write response: %w", err)
				}
				nWritten += n
			}
			writeBuffer.Reset()

			return nil
		}

		for _, req := range reqs {
			if err := handler(req, writeRespFunc); err != nil {
				return fmt.Errorf("failed to handle request: %w", err)
			}
		}

		return nil
	}

	return tcpHandler
}
