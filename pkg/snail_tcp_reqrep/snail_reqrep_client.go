package snail_tcp_reqrep

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_parser"
	"github.com/GiGurra/snail/pkg/snail_tcp"
	"io"
	"sync"
)

// ClientRespHandler is the custom response handler for a client connection.
type ClientRespHandler[Rep any] func(rep *Rep) error

type SnailClient[Req any, Resp any] struct {
	underlying  *snail_tcp.SnailClient
	handlerFunc ClientRespHandler[Resp]
	writeFunc   snail_parser.WriteFunc[Req]
	parseFunc   snail_parser.ParseFunc[Resp]
}

func NewClient[Req any, Resp any](
	tcpOpts *snail_tcp.SnailClientOpts,
	handlerFunc ClientRespHandler[Resp],
	writeFunc snail_parser.WriteFunc[Req],
	parseFunc snail_parser.ParseFunc[Resp],
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

func (s *SnailClient[Req, Resp]) Underlying() *snail_tcp.SnailServer {
	return s.underlying
}

func (s *SnailClient[Req, Resp]) Close() {
	s.underlying.Close()
}

func newTcpClientRespHandler[Req any, Resp any](
	newHandlerFunc func() ServerConnHandler[Req, Resp],
	parseFunc snail_parser.ParseFunc[Resp],
) snail_tcp.ServerConnHandler {

	handler := newHandlerFunc()
	writeBuffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
	writeMutex := sync.Mutex{}

	tcpHandler := func(readBuffer *snail_buffer.Buffer, writer io.Writer) error {

		if readBuffer == nil || writer == nil {
			return handler(nil, nil)
		}

		reqs, err := snail_parser.ParseAll[Req](readBuffer, parseFunc)
		if err != nil {
			return fmt.Errorf("failed to parse requests: %w", err)
		}

		// A mutex is likely to be faster than a channel here, since we are
		// dealing with n multiplexed requests over a single connection.
		// They are likely to be in the range of 1-1000.
		writeRespFunc := func(rep *Resp) error {
			writeMutex.Lock()
			defer writeMutex.Unlock()

			if err := writeFunc(writeBuffer, *rep); err != nil {
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
			if err := handler(&req, writeRespFunc); err != nil {
				return fmt.Errorf("failed to handle request: %w", err)
			}
		}

		return nil
	}

	return tcpHandler
}
