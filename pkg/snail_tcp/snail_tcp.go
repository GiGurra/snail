package snail_tcp

import (
	"errors"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"io"
)

func SendAll(socket io.Writer, data []byte) error {

	numBytesSent := 0

	for numBytesSent < len(data) {
		n, err := socket.Write(data[numBytesSent:])
		if err != nil {
			return fmt.Errorf("failed to write data: %w", err)
		}
		if n < 0 {
			return fmt.Errorf("failed to write data, n < 0")
		}
		numBytesSent += n
	}

	return nil
}

func ReadToBuffer(minBuf int, from io.Reader, to *snail_buffer.Buffer) error {

	to.EnsureSpareCapacity(minBuf)
	n, err := from.Read(to.UnderlyingWriteable())

	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	if n <= 0 {
		return errors.New("failed to read data, n <= 0")
	}

	to.AddWritten(n)

	return nil
}
