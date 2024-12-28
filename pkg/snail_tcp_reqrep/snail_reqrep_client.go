package snail_tcp_reqrep

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_parser"
	"github.com/GiGurra/snail/pkg/snail_tcp"
)

// ClientRespHandler is the custom response handler for a client connection.
type ClientRespHandler[Rep any] func(rep *Rep) error

type SnailClient[Req any, Resp any] struct {
	underlying *snail_tcp.SnailClient
	writeFunc  snail_parser.WriteFunc[Req]
	parseFunc  snail_parser.ParseFunc[Resp]
}

func NewClient[Req any, Resp any](
	ip string,
	port int,
	tcpOpts *snail_tcp.SnailClientOpts,
	handlerFunc ClientRespHandler[Resp],
	writeFunc snail_parser.WriteFunc[Req],
	parseFunc snail_parser.ParseFunc[Resp],
) (*SnailClient[Req, Resp], error) {

	underlying, err := snail_tcp.NewClient(ip, port, tcpOpts, newTcpClientRespHandler(handlerFunc, parseFunc))
	if err != nil {
		return nil, fmt.Errorf("failed to create underlying server: %w", err)
	}

	return &SnailClient[Req, Resp]{
		underlying: underlying,
		writeFunc:  writeFunc,
		parseFunc:  parseFunc,
	}, nil
}

func (s *SnailClient[Req, Resp]) Underlying() *snail_tcp.SnailClient {
	return s.underlying
}

func (s *SnailClient[Req, Resp]) Close() {
	s.underlying.Close()
}

func newTcpClientRespHandler[Resp any](
	respHandler func(resp *Resp) error,
	parseFunc snail_parser.ParseFunc[Resp],
) snail_tcp.ClientRespHandler {

	tcpHandler := func(readBuffer *snail_buffer.Buffer) error {

		if readBuffer == nil {
			return respHandler(nil)
		}

		reqs, err := snail_parser.ParseAll[Resp](readBuffer, parseFunc)
		if err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		for _, req := range reqs {
			if err := respHandler(&req); err != nil {
				return fmt.Errorf("failed to handle response: %w", err)
			}
		}

		return nil
	}

	return tcpHandler
}
