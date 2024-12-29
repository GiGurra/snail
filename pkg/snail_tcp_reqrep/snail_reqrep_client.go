package snail_tcp_reqrep

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_parser"
	"github.com/GiGurra/snail/pkg/snail_tcp"
	"sync"
)

type ClientStatus int

const (
	ClientStatusOK           ClientStatus = iota
	ClientStatusDisconnected ClientStatus = iota
)

// ClientRespHandler is the custom response handler for a client connection.
type ClientRespHandler[Resp any] func(resp Resp, tpe ClientStatus) error

type SnailClient[Req any, Resp any] struct {
	underlying *snail_tcp.SnailClient
	writeFunc  snail_parser.WriteFunc[Req]
	parseFunc  snail_parser.ParseFunc[Resp]
	writeMutex sync.Mutex
	convertBuf *snail_buffer.Buffer
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
		writeMutex: sync.Mutex{},
		convertBuf: snail_buffer.New(snail_buffer.BigEndian, 1024),
	}, nil
}

func (s *SnailClient[Req, Resp]) Underlying() *snail_tcp.SnailClient {
	return s.underlying
}

func (s *SnailClient[Req, Resp]) Close() {
	s.underlying.Close()
}

func (s *SnailClient[Req, Resp]) Send(r Req) error {
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()
	defer s.convertBuf.Reset()

	if err := s.writeFunc(s.convertBuf, r); err != nil {
		return fmt.Errorf("failed to serialize request: %w", err)
	}

	if err := s.underlying.SendBytes(s.convertBuf.UnderlyingReadable()); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	return nil
}

func (s *SnailClient[Req, Resp]) SendBatch(rs []Req) error {
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()
	return s.SendBatchUnsafe(rs)
}

// SendBatchUnsafe sends a batch of requests without locking the write mutex.
// This should only be used if the caller is sure that no other goroutine is
// using this client.
func (s *SnailClient[Req, Resp]) SendBatchUnsafe(rs []Req) error {
	defer s.convertBuf.Reset()

	for _, r := range rs {
		if err := s.writeFunc(s.convertBuf, r); err != nil {
			return fmt.Errorf("failed to serialize request: %w", err)
		}
	}

	if err := s.underlying.SendBytes(s.convertBuf.UnderlyingReadable()); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	return nil
}

func newTcpClientRespHandler[Resp any](
	respHandler ClientRespHandler[Resp],
	parseFunc snail_parser.ParseFunc[Resp],
) snail_tcp.ClientRespHandler {

	tcpHandler := func(readBuffer *snail_buffer.Buffer) error {

		if readBuffer == nil {
			var zero Resp
			return respHandler(zero, ClientStatusDisconnected)
		}

		reqs, err := snail_parser.ParseAll[Resp](readBuffer, parseFunc)
		if err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		for _, req := range reqs {
			if err := respHandler(req, ClientStatusOK); err != nil {
				return fmt.Errorf("failed to handle response: %w", err)
			}
		}

		return nil
	}

	return tcpHandler
}
