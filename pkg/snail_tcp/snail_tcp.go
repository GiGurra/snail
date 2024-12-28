package snail_tcp

import (
	"fmt"
	"io"
)

func SendAll(socket io.Writer, data []byte) error {

	numBytesSent := 0

	for numBytesSent < len(data) {
		n, err := socket.Write(data[numBytesSent:])
		if err != nil {
			return fmt.Errorf("failed to write data to socket: %w", err)
		}
		if n < 0 {
			return fmt.Errorf("failed to write data to socket, n < 0")
		}
		numBytesSent += n
	}

	return nil
}
