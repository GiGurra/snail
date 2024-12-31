package main

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_tcp"
	"log/slog"
	"net"
)

func main() {
	// This is a dummy http1.1 server
	port := 8080
	srv, err := snail_tcp.NewServer(nil, &snail_tcp.SnailServerOpts{Port: port})
	if err != nil {
		panic(fmt.Errorf("failed to create server: %w", err))
	}

	// Start the server
	slog.Info(fmt.Sprintf("Started server on port %d", srv.Port()))

	// Sleep forever
	select {}
}

func newHandlerFunc(conn net.Conn) snail_tcp.ServerConnHandler {
	return func(buf *snail_buffer.Buffer) error {
		return nil
	}
}
