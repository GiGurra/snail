package snail_tcp_reqrep

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_parser"
	"github.com/GiGurra/snail/pkg/snail_tcp"
	"io"
)

// ServerConnHandler is the custom handler for a server connection. If the socket is closed, nil, nil is called
type ServerConnHandler[Req any, Rep any] func(
	req *Req,
	repFunc func(rep *Rep) error,
) error

type SnailServer[Req any, Rep any] struct {
	underlying     *snail_tcp.SnailServer
	newHandlerFunc func() ServerConnHandler[Req, Rep]
	parseFunc      snail_parser.ParseFunc[Req]
	writeFunc      snail_parser.WriteFunc[Rep]
}

func NewServer[Req any, Rep any](
	newHandlerFunc func() ServerConnHandler[Req, Rep],
	tcpOpts *snail_tcp.SnailServerOpts,
	parseFunc snail_parser.ParseFunc[Req],
	writeFunc snail_parser.WriteFunc[Rep],
) (*SnailServer[Req, Rep], error) {

	newTcpHandlerFunc := func() snail_tcp.ServerConnHandler {
		return newTcpServerConnHandler[Req, Rep](newHandlerFunc, parseFunc, writeFunc)
	}

	underlying, err := snail_tcp.NewServer(newTcpHandlerFunc, tcpOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create underlying server: %w", err)
	}

	return &SnailServer[Req, Rep]{
		underlying:     underlying,
		newHandlerFunc: newHandlerFunc,
		parseFunc:      parseFunc,
		writeFunc:      writeFunc,
	}, nil
}

func (s *SnailServer[Req, Rep]) Underlying() *snail_tcp.SnailServer {
	return s.underlying
}

func (s *SnailServer[Req, Rep]) Close() {
	s.underlying.Close()
}

func newTcpServerConnHandler[Req any, Rep any](
	newHandlerFunc func() ServerConnHandler[Req, Rep],
	parseFunc snail_parser.ParseFunc[Req],
	writeFunc snail_parser.WriteFunc[Rep],
) snail_tcp.ServerConnHandler {

	handler := newHandlerFunc()
	writeBuffer := snail_buffer.New(snail_buffer.BigEndian, 1024)

	tcpHandler := func(readBuffer *snail_buffer.Buffer, writer io.Writer) error {

		if readBuffer == nil || writer == nil {
			return handler(nil, nil)
		}

		reqs, err := snail_parser.ParseAll[Req](readBuffer, parseFunc)
		if err != nil {
			return fmt.Errorf("failed to parse requests: %w", err)
		}

		// TODO: Handle multi threading for the response :S
		// channels? Something else?
		writeRespFunc := func(rep *Rep) error {
			if err := writeFunc(writeBuffer, *rep); err != nil {
				return fmt.Errorf("failed to write response: %w", err)
			}

			// Write the response
			start := writeBuffer.ReadPos()
			bytes := writeBuffer.Underlying()
			nWritten := 0
			for nWritten < writeBuffer.Readable() {
				n, err := writer.Write(bytes[start+nWritten:])
				if err != nil {
					return fmt.Errorf("failed to write response: %w", err)
				}
				nWritten += n
			}

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
