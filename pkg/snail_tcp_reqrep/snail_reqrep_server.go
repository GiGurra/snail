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

	tcpHandler := func(buffer *snail_buffer.Buffer, writer io.Writer) error {

		if buffer == nil || writer == nil {
			return handler(nil, nil)
		}

		reqs, err := snail_parser.ParseAll[Req](buffer, parseFunc)
		if err != nil {
			return fmt.Errorf("failed to parse requests: %w", err)
		}

		for _, req := range reqs {
			if err := handler(&req, func(rep *Rep) error {
				if err := writeFunc(buffer, *rep); err != nil {
					return fmt.Errorf("failed to write response: %w", err)
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to handle request: %w", err)
			}
		}

		return nil
	}

	return tcpHandler
}
