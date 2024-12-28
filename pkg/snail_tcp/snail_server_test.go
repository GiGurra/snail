package snail_tcp

import (
	"errors"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewServer_sendMessageTpServer(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("json", "info", false)

	recvCh := make(chan []byte)

	newHandlerFunc := func() ServerConnHandler {
		return func(buffer *snail_buffer.Buffer, writer io.Writer) error {
			if buffer == nil || writer == nil {
				slog.Info("Closing connection")
				return nil
			} else {
				slog.Info("Handler received data")
				recvCh <- buffer.ReadAll()
				return nil
			}
		}
	}

	server, err := NewServer(newHandlerFunc, nil)
	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}
	defer server.Close()

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	slog.Info("Listener port", slog.Int("port", server.Port()))
	client, err := NewClient("localhost", server.Port(), nil, func(buffer *snail_buffer.Buffer) error {
		slog.Info("Client received data")
		return nil
	})
	if err != nil {
		t.Fatalf("error creating client: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatalf("expected client, got nil")
	}

	err = client.SendBytes([]byte("Hello World!"))
	if err != nil {
		t.Fatalf("error sending msg: %v", err)
	}

	select {
	case msg := <-recvCh:
		if string(msg) != "Hello World!" {
			t.Fatalf("expected msg 'Hello World!', got '%s'", string(msg))
		}
		slog.Info("Received message", slog.String("msg", string(msg)))
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for message")
	}

}

func TestNewServer_send_1_GB(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("json", "info", false)

	numTotalMessages := 1_000_000_000
	batchSize := 100_000
	nBatches := numTotalMessages / batchSize

	if //goland:noinspection GoBoolExpressions
	numTotalMessages%batchSize != 0 {
		t.Fatalf("numTotalMessages must be divisible by batchSize")
	}

	atomicCounter := atomic.Int64{}
	recvSignal := make(chan byte, 10)
	batch := make([]byte, batchSize)

	newHandlerFunc := func() ServerConnHandler {
		return func(buffer *snail_buffer.Buffer, writer io.Writer) error {
			if buffer == nil || writer == nil {
				slog.Info("Closing connection")
				return nil
			} else {
				atomicCounter.Add(int64(buffer.NumBytesReadable()))
				buffer.Reset()
				//slog.Info("Handler received data")
				//for _, b := range buffer.ReadAll() {
				//	recvCh <- b
				//}
				recvSignal <- 1
				return nil
			}
		}
	}

	server, err := NewServer(newHandlerFunc, nil)
	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}
	defer server.Close()

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	slog.Info("Listener port", slog.Int("port", server.Port()))
	client, err := NewClient(
		"localhost",
		server.Port(),
		nil,
		func(buffer *snail_buffer.Buffer) error {
			slog.Error("Client received response data, should not happen in this test")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("error creating client: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatalf("expected client, got nil")
	}

	go func() {
		slog.Info("Sending all bytes")
		for i := 0; i < nBatches; i++ {
			err = client.SendBytes(batch)
			if err != nil {
				panic(fmt.Errorf("error sending msg: %w", err))
			}
		}
	}()

	for atomicCounter.Load() < int64(numTotalMessages) {
		select {
		case _ = <-recvSignal:
		case <-time.After(1 * time.Second):
			t.Fatalf("timeout waiting for message")
			return
		}
	}

	if atomicCounter.Load() != int64(numTotalMessages) {
		t.Fatalf("expected %d bytes, got %d", numTotalMessages, atomicCounter.Load())
	}

	slog.Info("Received all messages")

}

func TestNewServer_send_3_GB_n_threads(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("json", "info", false)

	numTotalMessages := 3_000_000_000
	batchSize := 100_000
	nGoRoutines := 16
	nBatchesTotal := numTotalMessages / batchSize
	nBatchesPerRoutine := nBatchesTotal / nGoRoutines
	tcpWindowSize := 512 * 1024 // Anything 64 kB or larger seems to have no effect. Below 64 kB performance drops quickly.

	slog.Info("numTotalMessages", slog.Int("numTotalMessages", numTotalMessages))
	slog.Info("batchSize", slog.Int("batchSize", batchSize))
	slog.Info("nGoRoutines", slog.Int("nGoRoutines", nGoRoutines))
	slog.Info("nBatchesTotal", slog.Int("nBatchesTotal", nBatchesTotal))
	slog.Info("nBatchesPerRoutine", slog.Int("nBatchesPerRoutine", nBatchesPerRoutine))
	slog.Info("tcpWindowSize", slog.Int("tcpWindowSize", tcpWindowSize))

	if //goland:noinspection GoBoolExpressions
	numTotalMessages%batchSize != 0 {
		t.Fatalf("numTotalMessages must be divisible by batchSize")
	}

	if //goland:noinspection GoBoolExpressions
	nBatchesPerRoutine*nGoRoutines != nBatchesTotal {
		t.Fatalf("nBatchesPerRoutine * nGoRoutines must be equal to nBatchesTotal")
	}

	atomicCounter := atomic.Int64{}
	recvSignal := make(chan byte, 10)
	batch := make([]byte, batchSize)

	newHandlerFunc := func() ServerConnHandler {
		return func(buffer *snail_buffer.Buffer, writer io.Writer) error {
			if buffer == nil || writer == nil {
				slog.Debug("Closing connection")
				return nil
			} else {
				atomicCounter.Add(int64(buffer.NumBytesReadable()))
				buffer.Reset()
				//slog.Info("Handler received data")
				//for _, b := range buffer.ReadAll() {
				//	recvCh <- b
				//}
				recvSignal <- 1
				return nil
			}
		}
	}

	server, err := NewServer(newHandlerFunc, &SnailServerOpts{
		TcpReadWindowSize: tcpWindowSize,
	})
	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}
	defer server.Close()

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	slog.Info("Listener port", slog.Int("port", server.Port()))

	slog.Info("Sending all bytes")

	t0 := time.Now()

	for i := 0; i < nGoRoutines; i++ {
		go func() {

			client, err := NewClient(
				"localhost",
				server.Port(),
				&SnailClientOpts{
					TcpSendWindowSize: tcpWindowSize,
				},
				func(buffer *snail_buffer.Buffer) error {
					slog.Error("Client received response data, should not happen in this test")
					return nil
				},
			)
			if err != nil {
				panic(fmt.Errorf("error creating client: %w", err))
			}
			defer client.Close()

			if client == nil {
				panic(fmt.Errorf("expected client, got nil"))
			}

			for i := 0; i < nBatchesPerRoutine; i++ {
				err = client.SendBytes(batch)
				if err != nil {
					panic(fmt.Errorf("error sending msg: %w", err))
				}
			}
		}()
	}

	for atomicCounter.Load() < int64(numTotalMessages) {
		select {
		case _ = <-recvSignal:
		case <-time.After(1 * time.Second):
			t.Fatalf("timeout waiting for message")
			return
		}
	}

	elapsed := time.Since(t0)

	if atomicCounter.Load() != int64(numTotalMessages) {
		t.Fatalf("expected %d bytes, got %d", numTotalMessages, atomicCounter.Load())
	}

	slog.Info("Received all messages")

	slog.Info(fmt.Sprintf("Elapsed time: %v", elapsed))

	bytesPerSecond := float64(numTotalMessages) / elapsed.Seconds()

	slog.Info(fmt.Sprintf("Bytes per second: %v", prettyInt3Digits(int64(bytesPerSecond))))
	slog.Info(fmt.Sprintf("Bits per second: %v", prettyInt3Digits(int64(bytesPerSecond*8))))

}

var prettyPrinter = message.NewPrinter(language.English)

func prettyInt3Digits(n int64) string {
	return prettyPrinter.Sprintf("%d", n)
}

func TestReferenceTcpPerf(t *testing.T) {
	// This test is a reference test the golang tcp performance.
	nClients := 16
	testTime := 1 * time.Second
	chunkSize := 512 * 1024
	readBufSize := 10 * chunkSize

	// Create a server socket
	socket, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("error creating server socket: %v", err)
	}
	defer func() { _ = socket.Close() }()

	port := socket.Addr().(*net.TCPAddr).Port

	slog.Info("Listener port", slog.Int("port", port))
	//
	//// create clients
	//clients := make([]*net.TCPConn, nClients)
	//for i := 0; i < nClients; i++ {
	//	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	//	if err != nil {
	//		t.Fatalf("error creating client: %v", err)
	//	}
	//	clients[i] = conn.(*net.TCPConn)
	//}
	//defer func() {
	//	for _, conn := range clients {
	//		if conn != nil {
	//			_ = conn.Close()
	//		}
	//	}
	//}()
	// create clients
	clients := make([]*SnailClient, nClients)
	for i := 0; i < nClients; i++ {
		client, err := NewClient(
			"localhost",
			port,
			//&SnailClientOpts{
			//	TcpSendWindowSize: tcpWindowSize,
			//},
			nil,
			func(buffer *snail_buffer.Buffer) error {
				slog.Error("Client received response data, should not happen in this test")
				return nil
			},
		)
		if err != nil {
			panic(fmt.Errorf("error creating client: %w", err))
		}
		clients[i] = client
	}
	defer func() {
		for _, conn := range clients {
			if conn != nil {
				conn.Close()
			}
		}
	}()

	slog.Info("Clients are connected")

	// accept connections
	accepted := make([]*net.TCPConn, nClients)
	for i := 0; i < nClients; i++ {
		conn, err := socket.Accept()
		if err != nil {
			t.Fatalf("error accepting connection: %v", err)
		}
		accepted[i] = conn.(*net.TCPConn)
	}

	totalReadBytes := atomic.Int64{}

	// Run all readers
	slog.Info("Connections are accepted, setting up reader goroutines")
	wgRead := sync.WaitGroup{}
	for _, conn := range accepted {
		wgRead.Add(1)
		go func() {
			defer wgRead.Done()
			nReadThisConn := int64(0)
			readBuf := snail_buffer.New(snail_buffer.BigEndian, readBufSize)
			for {
				//n, err := conn.Read(readBuf)
				n, err := conn.Read(readBuf.UnderlyingWriteable())
				readBuf.Reset()
				if n > 0 {
					nReadThisConn += int64(n)
				}
				if err != nil || n <= 0 {
					totalReadBytes.Add(nReadThisConn)
					if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
						slog.Debug("Connection is closed, shutting down accepted conn")
						return
					} else {
						slog.Error(fmt.Sprintf("error reading from connection: %v", err))
						return
					}
				}
			}
		}()
	}

	// Run all writers
	slog.Info("Starting test")
	t0 := time.Now()
	wgWrite := sync.WaitGroup{}
	for _, client := range clients {
		wgWrite.Add(1)
		go func() {

			writeBuf := snail_buffer.New(snail_buffer.BigEndian, chunkSize)
			for time.Since(t0) < testTime {

				if client == nil {
					panic(fmt.Errorf("expected client, got nil"))
				}

				bytes := writeBuf.UnderlyingWriteable()
				err = client.SendBytes(bytes)
				if err != nil {
					if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
						slog.Debug("Connection is closed, shutting down client")
						return
					} else {
						panic(fmt.Errorf("error writing to connection: %w", err))
					}
				}

				//bytes := writeBuf.UnderlyingWriteable()
				//nWritten := 0
				//for nWritten < len(bytes) {
				//	n, err := conn.Write(bytes)
				//	if err != nil {
				//		if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				//			slog.Debug("Connection is closed, shutting down client")
				//			return
				//		} else {
				//			panic(fmt.Errorf("error writing to connection: %w", err))
				//		}
				//	}
				//	nWritten += n
				//}
			}
			wgWrite.Done()
		}()
	}

	wgWrite.Wait()

	// close all writers
	for i, conn := range clients {
		if conn != nil {
			conn.Close()
			clients[i] = nil
		}
	}

	// wait for all readers to finish
	wgRead.Wait()

	slog.Info("Test done")

	slog.Info(fmt.Sprintf("Total bytes written: %v", prettyInt3Digits(totalReadBytes.Load())))

	byteRate := float64(totalReadBytes.Load()) / testTime.Seconds()
	bitRate := byteRate * 8

	slog.Info(fmt.Sprintf("Byte rate: %v", prettyInt3Digits(int64(byteRate))))
	slog.Info(fmt.Sprintf("Bit rate: %v", prettyInt3Digits(int64(bitRate))))
}
