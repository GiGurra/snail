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
type ServerConnHandler func(*snail_buffer.Buffer, io.Writer) error

type SnailServer struct {
	socket         net.Listener
	newHandlerFunc func() ServerConnHandler
	opts           SnailServerOpts
}

type SnailServerOpts struct {
	//MaxConnections int // TODO: implement support for this
	Optimization OptimizationType
	ReadBufSize  int
	Port         int
}

func (s SnailServerOpts) WithDefaults() SnailServerOpts {
	res := s
	if res.ReadBufSize == 0 {
		res.ReadBufSize = 2048
	}
	return res
}

func NewServer(
	newHandlerFunc func() ServerConnHandler,
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

	go res.Run()

	return res, nil
}

func (s *SnailServer) Port() int {
	return s.socket.Addr().(*net.TCPAddr).Port
}

func (s *SnailServer) Run() {

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

		slog.Debug("Accepted connection", slog.String("remote_addr", conn.RemoteAddr().String()))
		go s.loopConn(conn)
	}
}

func (s *SnailServer) Close() {
	err := s.socket.Close()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to close socket: %v", err))
	}
}

func (s *SnailServer) loopConn(conn net.Conn) {
	// read all messages see https://stackoverflow.com/questions/51046139/reading-data-from-socket-golang
	accumBuf := snail_buffer.New(snail_buffer.BigEndian, s.opts.ReadBufSize)
	handler := s.newHandlerFunc()

	giveUp := func() {
		err := conn.Close()
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to close connection: %v", err))
		}
		err = handler(nil, nil)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to handle connection after close: %v", err))
		}
	}

	for {

		accumBuf.EnsureSpareCapacity(s.opts.ReadBufSize)
		n, err := conn.Read(accumBuf.UnderlyingWriteable())
		if err != nil {
			// If EOF, it's a normal close
			if errors.Is(err, io.EOF) {
				slog.Debug("EOF, closing connection")
				giveUp()
				return
			} else {
				slog.Error(fmt.Sprintf("Failed to read from connection: %v", err))
				giveUp()
				return
			}
		}
		if n <= 0 {
			slog.Debug("No data read, closing connection")
			giveUp()
			return
		}
		accumBuf.AddWritten(n)

		slog.Debug(fmt.Sprintf("Read %d bytes", n))

		err = handler(accumBuf, conn)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to handle connection data: %v", err))
			giveUp()
			return
		}
	}
}
