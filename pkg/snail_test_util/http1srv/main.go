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
	srv, err := snail_tcp.NewServer(newHandlerFunc, &snail_tcp.SnailServerOpts{Port: port})
	if err != nil {
		panic(fmt.Errorf("failed to create server: %w", err))
	}

	// Start the server
	slog.Info(fmt.Sprintf("Started server on port %d", srv.Port()))

	// Sleep forever
	select {}
}

func newHandlerFunc(conn net.Conn) snail_tcp.ServerConnHandler {

	//writeBuf := snail_buffer.New(snail_buffer.LittleEndian, 64*1024)
	state := requestState{}

	return func(readBuf *snail_buffer.Buffer) error {

		if readBuf == nil {
			slog.Warn("Connection closed")
			return nil
		}

		// Read the request
		fmt.Println(string(readBuf.UnderlyingReadable()))
		fmt.Println("----")
		// Loop over lines
		newState := state
		bytes := readBuf.UnderlyingReadable()
		for i, b := range bytes {
			if b == '\r' {
				// ignore
				continue
			}

			if !newState.StartLineReading && !newState.StartLineReceived {
				// First character must be G for GET, the rest is ignored
				if b != 'G' {
					return fmt.Errorf("unsupported request method: %c", b)
				}
				newState.StartLineReading = true
				continue
			}

			if newState.StartLineReading {
				if b == '\n' {
					newState.StartLine = string(bytes[:i])
					fmt.Printf("Start line: %s\n", newState.StartLine)
					newState.StartLineReceived = true
					newState.StartLineReading = false
					newState.HeadersReading = true
					continue
				}
			}

			if newState.HeadersReading {
				if newState.CurrentHeaderStart <= 0 {
					newState.CurrentHeaderStart = i
				}
				if b == '\n' {
					// End of header
					newState.HeadersReceived = true
					headerLen := i - newState.CurrentHeaderStart
					if headerLen == 0 {
						// End of headers
						newState.HeadersReading = false
						newState.HeadersReceived = true
						continue
					} else {
						header := string(bytes[newState.CurrentHeaderStart:i])
						newState.Headers = append(newState.Headers, header)
						fmt.Printf("Header: %s\n", header)
						newState.CurrentHeaderStart = 0
					}
					continue
				}
			}

		}
		return nil
	}
}

type requestState struct {
	StartLineReading  bool
	StartLineReceived bool
	StartLine         string

	HeadersReading     bool
	HeadersReceived    bool
	CurrentHeaderStart int
	Headers            []string
}
