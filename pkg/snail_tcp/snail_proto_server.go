package custom_proto

import (
	"errors"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"log/slog"
	"net"
)

type CustomProtoServer struct {
	socket       net.Listener
	recvCh       chan []CustomProtoMsg
	optimization OptimizationType
}

func NewCustomProtoServer(
	port int,
	optimization OptimizationType,
) (*CustomProtoServer, error) {
	socket, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	res := &CustomProtoServer{
		socket:       socket,
		recvCh:       make(chan []CustomProtoMsg, 1000000),
		optimization: optimization,
	}

	go res.Run()

	return res, nil
}

func (s *CustomProtoServer) RecvCh() <-chan []CustomProtoMsg {
	return s.recvCh
}

func (s *CustomProtoServer) Port() int {
	return s.socket.Addr().(*net.TCPAddr).Port
}

func (c *CustomProtoServer) Run() {
	defer close(c.recvCh)
	for {
		conn, err := c.socket.Accept()
		if err != nil {
			// if is socket closed, exit
			if errors.Is(err, net.ErrClosed) {
				slog.Debug("Server socket is closed, shutting down CustomProtoServer")
				return
			}
			slog.Warn(fmt.Sprintf("Failed to accept connection: %v", err))
			continue
		}

		if c.optimization == OptimizeForThroughput {
			err = conn.(*net.TCPConn).SetNoDelay(false) // we favor latency over throughput here.
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to set TCP_NODELAY=false: %v. Proceeding anyway :S", err))
			}
		} else {
			err = conn.(*net.TCPConn).SetNoDelay(true) // we favor latency over throughput here.
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to set TCP_NODELAY=true: %v. Proceeding anyway :S", err))
			}
		}

		slog.Debug("Accepted connection", slog.String("remote_addr", conn.RemoteAddr().String()))
		go c.loopConn(conn)
	}
}

func (c *CustomProtoServer) Close() {
	err := c.socket.Close()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to close socket: %v", err))
	}
}

func (c *CustomProtoServer) loopConn(conn net.Conn) {
	// read all messages see https://stackoverflow.com/questions/51046139/reading-data-from-socket-golang
	readBuf := make([]byte, 1024)
	accumBuf := snail_buffer.New(snail_buffer.BigEndian, 1024)

	giveUp := func() {
		err := conn.Close()
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to close connection: %v", err))
		}
	}

	for {
		n, err := conn.Read(readBuf)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to read from connection: %v", err))
			giveUp()
			return
		}
		if n <= 0 {
			slog.Debug("No data read, closing connection")
			giveUp()
			return
		}
		slog.Debug(fmt.Sprintf("Read %d bytes", n))
		accumBuf.WriteBytes(readBuf[:n])
		msgs, err := TryReadMsgs(accumBuf)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to read messages: %v", err))
			giveUp()
			return
		}
		c.recvCh <- msgs
	}
}
