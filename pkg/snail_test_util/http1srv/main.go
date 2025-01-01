package main

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_batcher"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_tcp"
	"log/slog"
	"net"
	"strings"
	"time"
	"unicode"
)

func main() {
	// This is a dummy http1.1 server
	//port := 8080
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

	//writeBuf := snail_buffer.New(snail_buffer.LittleEndian, 64*1024)

	dateStr := time.Now().Format(time.RFC1123)
	defaultResponse := []byte(StripMargin(
		`|HTTP/1.1 200 OK
			 |Server: snail
			 |Date: ` + dateStr + `
			 |Connection: keep-alive
			 |Content-Length: 0
			 |
			 |`,
	))

	finalWriteBuf := snail_buffer.New(snail_buffer.LittleEndian, 64*1024)
	batcher := snail_batcher.NewSnailBatcher[[]byte](
		1,
		2,
		false,
		5*time.Minute, // dont need with batch size 1
		func(i [][]byte) error {

			if len(i) == 0 {
				return nil
			}

			if len(i) == 1 {
				err := snail_tcp.SendAll(conn, i[0])
				if err != nil {
					return fmt.Errorf("failed to send response: %w", err)
				}
				return nil
			}

			slog.Warn("Batch size > 1")

			finalWriteBuf.Reset()
			for _, b := range i {
				finalWriteBuf.WriteBytes(b)
			}
			err := snail_tcp.SendAll(conn, finalWriteBuf.Underlying())
			if err != nil {
				return fmt.Errorf("failed to send response: %w", err)
			}
			return nil
		},
	)

	makeWriteBufs := func(n int) []*snail_buffer.Buffer {
		res := make([]*snail_buffer.Buffer, n)
		for i := 0; i < n; i++ {
			res[i] = snail_buffer.New(snail_buffer.LittleEndian, 64*1024)
		}
		return res
	}

	writeBufs := makeWriteBufs(4)

	nextWriteBuf := 0

	getNextWriteBuf := func() *snail_buffer.Buffer {
		res := writeBufs[nextWriteBuf]
		nextWriteBuf++
		if nextWriteBuf == len(writeBufs) {
			nextWriteBuf = 0
		}
		return res
	}

	return func(readBuf *snail_buffer.Buffer) error {

		if readBuf == nil {
			batcher.Close()
			//slog.Warn("Connection closed")
			return nil
		}

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
					//fmt.Printf("Start line: %s\n", state.StartLine)
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
						//fmt.Println("End of headers")
						state.RequestComplete = true
						responsesToSend++
						state = getRequestState{}
						readBuf.SetReadPos(i + 1)
						continue
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

		writeBuf := getNextWriteBuf()
		writeBuf.Reset()
		for i := 0; i < responsesToSend; i++ {
			writeBuf.WriteBytes(defaultResponse)
		}
		//
		//The batcher is kind of pointless here, as the incoming data by h2load is already so batched
		//and pipelined, that we are already by default sending in the optimal batch size to the socket api.
		//batcher.Add(writeBuf.Underlying())
		err := snail_tcp.SendAll(conn, writeBuf.Underlying())
		if err != nil {
			return fmt.Errorf("failed to send response: %w", err)
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

// StripMarginWith Accepts a string and a `marginChar` rune to strip margins
// from a multiline string similar to Scala's stripMargin method
func StripMarginWith(str string, marginChar rune) string {
	lines := strings.Split(str, "\n")

	for i, line := range lines {
		strippedLine := strings.TrimLeftFunc(line, unicode.IsSpace)
		if len(strippedLine) > 0 && strippedLine[0] == byte(marginChar) {
			strippedLine = strippedLine[1:]
		}

		lines[i] = strippedLine
	}

	return strings.Join(lines, "\n")
}

// StripMargin Accepts a string and strips margins from a multiline string
// using `|` similar to Scala's stripMargin method
func StripMargin(str string) string {
	return StripMarginWith(str, '|')
}

func copyBytes(src []byte) []byte {
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}
