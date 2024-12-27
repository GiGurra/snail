package snail_tcp_reqrep

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_tcp"
)

// ServerConnHandler is the custom handler for a server connection. If the socket is closed, nil, nil is called
type ServerConnHandler[Req any, Rep any] func(
	req *Req,
	repFunc func(rep *Rep) error,
) error

type SnailServer[Req any, Rep any] struct {
	underlying     *snail_tcp.SnailServer
	newHandlerFunc func() ServerConnHandler[Req, Rep]
}

func NewServer[Req any, Rep any](
	newHandlerFunc func() ServerConnHandler[Req, Rep],
	tcpOpts *snail_tcp.SnailServerOpts,
) (*SnailServer[Req, Rep], error) {

	// TODO: Implement the handler func bridge and worker pool

	underlying, err := snail_tcp.NewServer(nil, tcpOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create underlying server: %w", err)
	}

	return &SnailServer[Req, Rep]{
		underlying:     underlying,
		newHandlerFunc: newHandlerFunc,
	}, nil
}

func (s *SnailServer[Req, Rep]) Underlying() *snail_tcp.SnailServer {
	return s.underlying
}

func (s *SnailServer[Req, Rep]) Close() {
	s.underlying.Close()
}
