package snail_legacy_prototype

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"log/slog"
	"net"
)

type CustomProtoClient struct {
	socket net.Conn
	sndBuf *snail_buffer.Buffer
}

func NewCustomProtoClient(
	ip string,
	port int,
	optimization OptimizationType,
) (*CustomProtoClient, error) {
	socket, err := net.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}
	if optimization == OptimizeForThroughput {
		err = socket.(*net.TCPConn).SetNoDelay(false) // we favor throughput over latency here. This gets us massive speedups. (300k msgs/s -> 1m)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to set TCP_NODELAY=false: %v. Proceeding anyway :S", err))
		}
	} else {
		err = socket.(*net.TCPConn).SetNoDelay(true) // we favor latency over throughput here.
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to set TCP_NODELAY=true: %v. Proceeding anyway :S", err))
		}
	}
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to set TCP_NODELAY: %v. Proceeding anyway :S", err))
	}
	return &CustomProtoClient{
		socket: socket,
		sndBuf: snail_buffer.New(snail_buffer.BigEndian, 1024),
	}, nil
}

func (c *CustomProtoClient) Close() {
	err := c.socket.Close()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to close socket: %v", err))
	}
}

func (c *CustomProtoClient) SendMsg(msg CustomProtoMsg) error {

	c.sndBuf.Reset()
	err := WriteMsg(c.sndBuf, msg)
	if err != nil {
		return fmt.Errorf("failed to serialize msg: %w", err)
	}
	nWritten := 0
	for nWritten < len(c.sndBuf.Underlying()) {
		n, err := c.socket.Write(c.sndBuf.Underlying()[nWritten:])
		if err != nil {
			return fmt.Errorf("failed to write msg to socket: %w", err)
		}
		if n < 0 {
			return fmt.Errorf("failed to write msg to socket, n < 0")
		}
		nWritten += n
	}

	return nil
}

func (c *CustomProtoClient) SendMsgs(msgs []CustomProtoMsg) error {

	c.sndBuf.Reset()
	for _, msg := range msgs {
		err := WriteMsg(c.sndBuf, msg)
		if err != nil {
			return fmt.Errorf("failed to serialize msg: %w", err)
		}
	}
	nWritten := 0
	for nWritten < len(c.sndBuf.Underlying()) {
		n, err := c.socket.Write(c.sndBuf.Underlying()[nWritten:])
		if err != nil {
			return fmt.Errorf("failed to write msg to socket: %w", err)
		}
		if n < 0 {
			return fmt.Errorf("failed to write msg to socket, n < 0")
		}
		nWritten += n
	}

	return nil
}
