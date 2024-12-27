package snail_legacy_prototype

import (
	"errors"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"log/slog"
	"net"
)

type OptimizationType int

const (
	OptimizeForLatency OptimizationType = iota
	OptimizeForThroughput
)

type CustomProtoTestServer struct {
	socket       net.Listener
	recvCh       chan []CustomProtoMsg
	optimization OptimizationType
}

func NewCustomProtoServer(
	port int,
	optimization OptimizationType,
) (*CustomProtoTestServer, error) {
	socket, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	res := &CustomProtoTestServer{
		socket:       socket,
		recvCh:       make(chan []CustomProtoMsg, 1000000),
		optimization: optimization,
	}

	go res.Run()

	return res, nil
}

func (s *CustomProtoTestServer) RecvCh() <-chan []CustomProtoMsg {
	return s.recvCh
}

func (s *CustomProtoTestServer) Port() int {
	return s.socket.Addr().(*net.TCPAddr).Port
}

func (s *CustomProtoTestServer) Run() {
	defer close(s.recvCh)
	for {
		conn, err := s.socket.Accept()
		if err != nil {
			// if is socket closed, exit
			if errors.Is(err, net.ErrClosed) {
				slog.Debug("Server socket is closed, shutting down CustomProtoTestServer")
				return
			}
			slog.Warn(fmt.Sprintf("Failed to accept connection: %v", err))
			continue
		}

		if s.optimization == OptimizeForThroughput {
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
		go s.loopConn(conn)
	}
}

func (s *CustomProtoTestServer) Close() {
	err := s.socket.Close()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to close socket: %v", err))
	}
}

func (s *CustomProtoTestServer) loopConn(conn net.Conn) {
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
		messages, err := TryReadMsgs(accumBuf)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to read messages: %v", err))
			giveUp()
			return
		}
		s.recvCh <- messages
	}
}
