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

	return func(readBuf *snail_buffer.Buffer) error {

		if readBuf == nil {
			slog.Warn("Connection closed")
			return nil
		}

		// Read the request
		//fmt.Println(string(readBuf.UnderlyingReadable()))
		//fmt.Println("----")
		// Loop over lines
		state := getRequestState{}
		bytes := readBuf.Underlying()
		responsesToSend := 0
		for i := readBuf.ReadPos(); i < len(bytes); i++ {
			b := bytes[i]
			if b == '\r' {
				// ignore
				continue
			}

			if !state.StartLineReading && !state.StartLineReceived {
				// First character must be G for GET, the rest is ignored
				if b != 'G' {
					return fmt.Errorf("unsupported request method: %c", b)
				}
				state.StartLineStart = i
				state.StartLineReading = true
				continue
			}

			if state.StartLineReading {
				if b == '\n' {
					state.StartLine = string(bytes[state.StartLineStart:i])
					fmt.Printf("Start line: %s\n", state.StartLine)
					state.StartLineReceived = true
					state.StartLineReading = false
					state.HeadersReading = true
					continue
				}
			}

			if state.HeadersReading {
				if state.CurrentHeaderStart <= 0 {
					state.CurrentHeaderStart = i
				}
				if b == '\n' {
					// End of header
					state.HeadersReceived = true
					headerLen := i - state.CurrentHeaderStart
					if headerLen == 0 {
						// End of headers
						state.HeadersReading = false
						state.HeadersReceived = true
						fmt.Println("End of headers")
						state.RequestComplete = true
						// TODO: Handle request
						responsesToSend++
						state = getRequestState{}
						// forward the read position
						readBuf.SetReadPos(i + 1)
						readBuf.DiscardReadBytes()
						continue
					} else {
						header := string(bytes[state.CurrentHeaderStart:i])
						state.Headers = append(state.Headers, header)
						fmt.Printf("Header: %s\n", header)
						state.CurrentHeaderStart = 0
					}
					continue
				}
			}

		}
		return nil
	}
}

type getRequestState struct {
	StartLineReading  bool
	StartLineReceived bool
	StartLineStart    int
	StartLine         string

	HeadersReading     bool
	HeadersReceived    bool
	CurrentHeaderStart int
	Headers            []string

	RequestComplete bool
}
