package main

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_batcher"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_tcp"
	"github.com/GiGurra/snail/pkg/snail_test_util/strutil"
	"log/slog"
	"net"
	"time"
)

func main() {
	port := 9080
	srv, err := snail_tcp.NewServer(newHandlerFunc, &snail_tcp.SnailServerOpts{
		Port:        port,
		ReadBufSize: 64 * 1024,
	})
	if err != nil {
		panic(fmt.Errorf("failed to create server: %w", err))
	}

	// Start the server
	slog.Info(fmt.Sprintf("Started server on port %d", srv.Port()))

	// Sleep forever
	select {}
}

func newHandlerFunc(conn net.Conn) snail_tcp.ServerConnHandler {

	writeBuf := snail_buffer.New(snail_buffer.BigEndian, 64*1024)

	dateStr := time.Now().Format(time.RFC1123)
	defaultResponse := []byte(strutil.StripMargin(
		`|HTTP/1.1 200 OK
			 |Server: snail
			 |Date: ` + dateStr + `
			 |Connection: keep-alive
			 |Content-Length: 0
			 |
			 |`,
	))

	batcher := snail_batcher.NewSnailBatcher[byte](
		64*1024,
		2*64*1024,
		false,
		5*time.Millisecond,
		func(bytes []byte) error {

			if len(bytes) == 0 {
				return nil
			}

			err := snail_tcp.SendAll(conn, bytes)
			if err != nil {
				return fmt.Errorf("failed to send response: %w", err)
			}
			return nil
		},
	)

	return func(readBuf *snail_buffer.Buffer) error {

		// Callback to indicate that the connection is closed
		if readBuf == nil {
			batcher.Close()
			//slog.Warn("Connection closed")
			return nil
		}

		state := parserState{}        // keep track of what we are doing
		bytes := readBuf.Underlying() // where we read from
		byteLen := len(bytes)         // maybe perf incr? :D to avoid calling len() all the time

		responsesToSend := 0
		for i := readBuf.ReadPos(); i < byteLen; i++ {

			b := bytes[i]

			if b == '\r' { // ignore
				continue
			}

			switch state.Status {
			case LookingForStartLine:
				if b != 'G' {
					return fmt.Errorf("unsupported request method: %c", b)
				}
				state.StartLineStart = i
				state.Status = ReadingStartLine
			case ReadingStartLine:
				if b == '\n' {
					state.StartLine = string(bytes[state.StartLineStart:i])
					//fmt.Printf("Start line: %s\n", state.StartLine)
					state.Status = ReadingHeaders
				}
			case ReadingHeaders:
				if state.CurrentHeaderStart <= 0 {
					state.CurrentHeaderStart = i
				}
				if b == '\n' {
					// End of header
					headerLen := i - state.CurrentHeaderStart
					if headerLen == 0 {
						// End of headers
						//fmt.Println("End of headers")
						responsesToSend++
						state = parserState{}
						readBuf.SetReadPos(i + 1)
					} else {
						header := string(bytes[state.CurrentHeaderStart:i])
						state.Headers = append(state.Headers, header)
						//fmt.Printf("Header: %s\n", header)
						state.CurrentHeaderStart = 0
					}
					continue
				}
			}
		}

		//fmt.Printf("Responses to send: %d\n", responsesToSend)

		/////////////////////////////////////////////////////////////////////
		// BATCHER Implementation
		// The batcher is kind of pointless here, as the incoming data by h2load is already so batched
		// and pipelined, that we are already by default sending in the optimal batch size to the socket api.
		// for i := 0; i < responsesToSend; i++ {
		//	 batcher.AddMany(defaultResponse)
		// }
		/////////////////////////////////////////////////////////////////////

		/////////////////////////////////////////////////////////////////////
		// NAIVE Implementation
		writeBuf.Reset()
		for i := 0; i < responsesToSend; i++ {
			writeBuf.WriteBytes(defaultResponse)
		}

		err := snail_tcp.SendAll(conn, writeBuf.Underlying())
		if err != nil {
			return fmt.Errorf("failed to send response: %w", err)
		}
		/////////////////////////////////////////////////////////////////////

		return nil
	}
}

type Status int

const (
	LookingForStartLine Status = iota
	ReadingStartLine    Status = iota
	ReadingHeaders
)

type parserState struct {
	Status Status

	StartLineStart int
	StartLine      string

	CurrentHeaderStart int
	Headers            []string
}
