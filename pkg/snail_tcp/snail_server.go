package snail_tcp

import (
	"errors"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"io"
	"log/slog"
	"net"
)

// ServerConnHandler is the custom handler for a server connection. If the socket is closed, nil, nil is called
type ServerConnHandler func(*snail_buffer.Buffer) error

type SnailServer struct {
	socket         net.Listener
	newHandlerFunc func(conn net.Conn) ServerConnHandler
	opts           SnailServerOpts
}

type SnailServerOpts struct {
	//MaxConnections int // TODO: implement support for this
	Optimization       OptimizationType
	ReadBufSize        int // this=initial size. TODO: implement max-grow-size
	Port               int
	TcpReadWindowSize  int
	TcpWriteWindowSize int
}

func (s SnailServerOpts) WithDefaults() SnailServerOpts {
	res := s
	if res.ReadBufSize == 0 {
		res.ReadBufSize = 64 * 1024
	}
	return res
}

func NewServer(
	newHandlerFunc func(conn net.Conn) ServerConnHandler,
	optsPtr *SnailServerOpts,
) (*SnailServer, error) {

	opts := func() SnailServerOpts {
		if optsPtr == nil {
			return SnailServerOpts{}
		}
		return *optsPtr
	}().WithDefaults()

	socket, err := net.Listen("tcp", fmt.Sprintf(":%d", opts.Port))
	if err != nil {
		return nil, err
	}

	res := &SnailServer{
		socket:         socket,
		newHandlerFunc: newHandlerFunc,
		opts:           opts,
	}

	go res.loopConnections()

	return res, nil
}

func (s *SnailServer) Port() int {
	return s.socket.Addr().(*net.TCPAddr).Port
}

func (s *SnailServer) loopConnections() {

	for {
		conn, err := s.socket.Accept()
		if err != nil {
			// if is socket closed, exit
			if errors.Is(err, net.ErrClosed) {
				slog.Debug("Server socket is closed, shutting down server")
				return
			}
			slog.Warn(fmt.Sprintf("Failed to accept connection: %v", err))
			continue
		}

		if s.opts.Optimization == OptimizeForThroughput {
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

		if s.opts.TcpReadWindowSize > 0 {
			err = conn.(*net.TCPConn).SetReadBuffer(s.opts.TcpReadWindowSize)
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to set TCP read buffer size: %v. Proceeding anyway :S", err))
			}
		}

		if s.opts.TcpWriteWindowSize > 0 {
			err = conn.(*net.TCPConn).SetWriteBuffer(s.opts.TcpWriteWindowSize)
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to set TCP write buffer size: %v. Proceeding anyway :S", err))
			}
		}

		slog.Debug("Accepted connection", slog.String("remote_addr", conn.RemoteAddr().String()))
		go s.loopConnection(conn)
	}
}

func (s *SnailServer) Close() {
	err := s.socket.Close()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to close socket: %v", err))
	}
}

func (s *SnailServer) loopConnection(conn net.Conn) {
	// read all messages see https://stackoverflow.com/questions/51046139/reading-data-from-socket-golang
	accumBuf := snail_buffer.New(snail_buffer.BigEndian, s.opts.ReadBufSize)
	handler := s.newHandlerFunc(conn)

	defer func() {
		err := conn.Close()
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to close connection: %v", err))
		}
		err = handler(nil)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to handle connection after close: %v", err))
		}
	}()

	for {

		err := ReadToBuffer(s.opts.ReadBufSize/5, conn, accumBuf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				slog.Debug("EOF, closing connection")
				return
			} else {
				slog.Error(fmt.Sprintf("Failed to read from connection: %v", err))
				return
			}
		}

		err = handler(accumBuf)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to handle connection data: %v", err))
			return
		}
		accumBuf.DiscardReadBytes()
	}
}
