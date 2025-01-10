package main

import (
	"errors"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_tcp"
	"github.com/GiGurra/snail/pkg/snail_tcp/manual_perf_tests/common"
	"io"
	"log/slog"
	"net"
	"strconv"
)

func main() {
	// This test is a reference test the golang tcp performance.

	// Create a server socket
	socket, err := net.Listen("tcp", ":"+strconv.Itoa(common.Port))
	if err != nil {
		panic(fmt.Sprintf("error creating server socket: %v", err))
	}
	defer func() { _ = socket.Close() }()

	port := socket.Addr().(*net.TCPAddr).Port

	slog.Info("Listener port", slog.Int("port", port))

	slog.Info("Waiting for clients to connect")
	for {
		conn, err := socket.Accept()
		if err != nil {
			panic(fmt.Sprintf("error accepting connection: %v", err))
		}
		slog.Info("Client connected")
		client := conn.(*net.TCPConn)
		// set window to tcpWindowSize
		if //goland:noinspection GoBoolExpressions
		common.TcpWindowSize > 0 {
			err = client.SetReadBuffer(common.TcpWindowSize)
			if err != nil {
				panic(fmt.Sprintf("error setting read buffer: %v", err))
			}
		}
		// Enable no delay
		//goland:noinspection GoBoolExpressions
		if common.Optimization == snail_tcp.OptimizeForThroughput {
			err = client.SetNoDelay(false)
		} else {
			err = client.SetNoDelay(true)
		}

		go keepReading(conn)
	}

	select {} // Wait forever

}

func keepReading(conn net.Conn) {
	nReadThisConn := int64(0)
	readBuf := make([]byte, common.ReadBufSize)
	for {
		n, err := conn.Read(readBuf)
		if n > 0 {
			nReadThisConn += int64(n)
		}
		if err != nil || n <= 0 {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				slog.Debug("Connection is closed, shutting down accepted conn")
				return
			} else {
				slog.Error(fmt.Sprintf("error reading from connection: %v", err))
				return
			}
		}
	}
}
