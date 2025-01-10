package snail_tcp

import (
	"errors"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"github.com/GiGurra/snail/pkg/snail_test_util"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
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

	newHandlerFunc := func(_ net.Conn) ServerConnHandler {
		return func(buffer *snail_buffer.Buffer) error {
			if buffer == nil {
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

	newHandlerFunc := func(_ net.Conn) ServerConnHandler {
		return func(buffer *snail_buffer.Buffer) error {
			if buffer == nil {
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
		case <-recvSignal:
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

	testTime := 1 * time.Second
	nGoRoutines := 16
	writeChunkSize := 512 * 1024
	readBufSize := 10 * writeChunkSize
	//tcpWindowSize := 512 * 1024 // Anything 64 kB or larger seems to have no effect. Below 64 kB performance drops quickly.

	slog.Info("writeChunkSize", slog.Int("writeChunkSize", writeChunkSize))
	slog.Info("readBufSize", slog.Int("readBufSize", readBufSize))
	slog.Info("nGoRoutines", slog.Int("nGoRoutines", nGoRoutines))
	//slog.Info("tcpWindowSize", slog.Int("tcpWindowSize", tcpWindowSize))

	atomicCounter := atomic.Int64{}

	wrReaders := sync.WaitGroup{}
	newHandlerFunc := func(_ net.Conn) ServerConnHandler {
		counter := int64(0)
		wrReaders.Add(1)
		return func(buffer *snail_buffer.Buffer) error {
			if buffer == nil {
				atomicCounter.Add(counter)
				wrReaders.Done()
				slog.Debug("Closing connection")
				return nil
			} else {
				counter += int64(buffer.NumBytesReadable())
				buffer.Reset()
				return nil
			}
		}
	}

	server, err := NewServer(newHandlerFunc, &SnailServerOpts{
		ReadBufSize: readBufSize,
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

	wgWriters := sync.WaitGroup{}
	for i := 0; i < nGoRoutines; i++ {
		wgWriters.Add(1)
		go func() {

			defer wgWriters.Done()

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
				panic(fmt.Errorf("error creating client: %w", err))
			}
			defer client.Close()

			if client == nil {
				panic(fmt.Errorf("expected client, got nil"))
			}

			writeBuf := snail_buffer.New(snail_buffer.BigEndian, writeChunkSize)
			for time.Since(t0) < testTime {
				err = client.SendBytes(writeBuf.UnderlyingWriteable())
				if err != nil {
					panic(fmt.Errorf("error sending msg: %w", err))
				}
			}
		}()
	}

	wgWriters.Wait()

	slog.Info("All writers are done")

	elapsed := time.Since(t0)

	slog.Info("Waiting for all readers to finish")
	wrReaders.Wait()

	slog.Info("Received all messages")
	numTotalMessages := atomicCounter.Load()

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
	nClients := 32
	testTime := 1 * time.Second
	writeBufSize := 10 * 128 * 1024
	readBufSize := writeBufSize
	tcpWindowSize := 10 * 128 * 1024
	optimization := OptimizeForThroughput

	// Create a server socket
	socket, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("error creating server socket: %v", err)
	}
	defer func() { _ = socket.Close() }()

	port := socket.Addr().(*net.TCPAddr).Port

	slog.Info("Listener port", slog.Int("port", port))

	// create clients
	clients := make([]net.Conn, nClients)
	writeBufs := make([][]byte, nClients)
	readBufs := make([][]byte, nClients)
	for i := 0; i < nClients; i++ {
		writeBufs[i] = make([]byte, writeBufSize)
		readBufs[i] = make([]byte, readBufSize)
		var err error
		clients[i], err = net.Dial("tcp", fmt.Sprintf("%s:%d", "localhost", port))
		if err != nil {
			t.Fatalf("error connecting to server: %v", err)
		}
		if //goland:noinspection GoBoolExpressions
		optimization == OptimizeForThroughput {
			err = clients[i].(*net.TCPConn).SetNoDelay(false) // we favor throughput over latency here. This gets us massive speedups. (300k msgs/s -> 1m)
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to set TCP_NODELAY=false: %v. Proceeding anyway :S", err))
			}
		} else {
			err = clients[i].(*net.TCPConn).SetNoDelay(true) // we favor latency over throughput here.
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to set TCP_NODELAY=true: %v. Proceeding anyway :S", err))
			}
		}
		//goland:noinspection GoBoolExpressions
		if tcpWindowSize > 0 {
			err = clients[i].(*net.TCPConn).SetWriteBuffer(tcpWindowSize)
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to set TCP send window size: %v. Proceeding anyway :S", err))
			}
		}
		if err != nil {
			panic(fmt.Errorf("error creating client: %w", err))
		}
	}
	defer func() {
		for _, conn := range clients {
			if conn != nil {
				_ = conn.Close()
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
		// set window to tcpWindowSize
		if //goland:noinspection GoBoolExpressions
		tcpWindowSize > 0 {
			err = accepted[i].SetReadBuffer(tcpWindowSize)
			if err != nil {
				t.Fatalf("error setting read buffer: %v", err)
			}
		}
		// Enable no delay
		//goland:noinspection GoBoolExpressions
		if optimization == OptimizeForThroughput {
			err = accepted[i].SetNoDelay(false)
		} else {
			err = accepted[i].SetNoDelay(true)
		}
	}

	totalReadBytes := atomic.Int64{}

	// Run all readers
	slog.Info("Connections are accepted, setting up reader goroutines")
	wgRead := sync.WaitGroup{}
	for i, conn := range accepted {
		wgRead.Add(1)
		readBuf := readBufs[i]
		go func() {
			defer wgRead.Done()
			nReadThisConn := int64(0)
			for {
				n, err := conn.Read(readBuf)
				if n > 0 {
					nReadThisConn += int64(n)
				}
				if err != nil || n <= 0 {
					if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
						totalReadBytes.Add(nReadThisConn)
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

	keepRunning := snail_test_util.TrueForTimeout(testTime)

	// Run all writers
	slog.Info("Starting test")
	t0 := time.Now()
	wgWrite := sync.WaitGroup{}
	totalWrittenBytes := atomic.Int64{}
	for i, client := range clients {
		wgWrite.Add(1)
		writeBuf := writeBufs[i]
		go func() {

			nReadThisConn := int64(0)

			for keepRunning.Load() {

				if client == nil {
					panic(fmt.Errorf("expected client, got nil"))
				}

				n, err := client.Write(writeBuf)
				if err != nil {
					if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
						slog.Debug("Connection is closed, shutting down client")
						return
					} else {
						panic(fmt.Errorf("error writing to connection: %w", err))
					}
				}
				if n > 0 {
					nReadThisConn += int64(n)
				}

			}
			totalWrittenBytes.Add(nReadThisConn)
			wgWrite.Done()
		}()
	}

	wgWrite.Wait()
	elapsed := time.Since(t0)

	// close all writers
	for i, conn := range clients {
		if conn != nil {
			_ = conn.Close()
			clients[i] = nil
		}
	}

	// wait for all readers to finish
	wgRead.Wait()

	slog.Info("Test done")

	slog.Info(fmt.Sprintf("Total bytes written: %v", prettyInt3Digits(totalWrittenBytes.Load())))
	slog.Info(fmt.Sprintf("Total bytes received: %v", prettyInt3Digits(totalReadBytes.Load())))

	byteRate := float64(totalReadBytes.Load()) / elapsed.Seconds()
	bitRate := byteRate * 8

	slog.Info(fmt.Sprintf("Byte rate: %v", prettyInt3Digits(int64(byteRate))))
	slog.Info(fmt.Sprintf("Bit rate: %v", prettyInt3Digits(int64(bitRate))))
}

func TestTcpRefReqRespRate(t *testing.T) {
	// This test is a reference test the golang tcp performance.
	testTime := 1 * time.Second
	clientBlob := make([]byte, 4) // an int32

	// Create a server socket
	socket, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("error creating server socket: %v", err)
	}
	defer func() { _ = socket.Close() }()

	// create listener goroutine
	go func() {
		conn, err := socket.Accept()
		if err != nil {
			panic(fmt.Errorf("error accepting connection: %w", err))
		}
		defer func() { _ = conn.Close() }()

		serverBlob := make([]byte, 4)
		for {
			// read
			n, err := conn.Read(serverBlob)
			if err != nil {
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					slog.Debug("Connection is closed, shutting down accepted conn")
					return
				} else {
					panic(fmt.Errorf("error reading from connection: %w", err))
				}
			}
			if n != 4 {
				panic(fmt.Errorf("expected to read 4 bytes, got %d", n))
			}
			// write it back
			n, err = conn.Write(serverBlob)
			if err != nil {
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					slog.Debug("Connection is closed, shutting down accepted conn")
					return
				} else {
					panic(fmt.Errorf("error writing to connection: %w", err))
				}
			}
			if n != 4 {
				panic(fmt.Errorf("expected to write 4 bytes, got %d", n))
			}
		}
	}()

	port := socket.Addr().(*net.TCPAddr).Port

	slog.Info("Listener port", slog.Int("port", port))

	clientSocket, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatalf("error connecting to server: %v", err)
	}

	t0 := time.Now()
	nReqs := int64(0)
	for time.Since(t0) < testTime {
		// write
		n, err := clientSocket.Write(clientBlob)
		if err != nil {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				slog.Debug("Connection is closed, shutting down client")
				return
			} else {
				panic(fmt.Errorf("error writing to connection: %w", err))
			}
		}
		if n != 4 {
			panic(fmt.Errorf("expected to write 4 bytes, got %d", n))
		}
		// read
		n, err = clientSocket.Read(clientBlob)
		if err != nil {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				slog.Debug("Connection is closed, shutting down client")
				return
			} else {
				panic(fmt.Errorf("error reading from connection: %w", err))
			}
		}
		if n != 4 {
			panic(fmt.Errorf("expected to read 4 bytes, got %d", n))
		}
		nReqs++
	}

	slog.Info("Test done")

	slog.Info(fmt.Sprintf("Total requests: %v", prettyInt3Digits(nReqs)))

}

func TestTcpRefReqRespRate_routines(t *testing.T) {
	// This test is a reference test the golang tcp performance.
	nGoRoutines := 512
	testTime := 1 * time.Second
	tcpNoDelay := true

	// Create a server socket
	socket, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("error creating server socket: %v", err)
	}
	defer func() { _ = socket.Close() }()

	// create listener goroutines
	go func() {
		for {
			conn, err := socket.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					slog.Debug("Listener is closed, shutting down")
					return
				} else {
					panic(fmt.Errorf("error accepting connection: %w", err))
				}
			}

			// set tcp no delay
			err = conn.(*net.TCPConn).SetNoDelay(tcpNoDelay)
			if err != nil {
				panic(fmt.Errorf("error setting no delay: %w", err))
			}

			// send window size
			err = conn.(*net.TCPConn).SetWriteBuffer(4)
			if err != nil {
				panic(fmt.Errorf("error setting write buffer: %w", err))
			}

			// set read buffer size
			err = conn.(*net.TCPConn).SetReadBuffer(4)
			if err != nil {
				panic(fmt.Errorf("error setting read buffer: %w", err))
			}

			go func() {
				serverBlob := make([]byte, 4)
				for {
					// read
					n, err := conn.Read(serverBlob)
					if err != nil {
						if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
							slog.Debug("Connection is closed, shutting down accepted conn")
							return
						} else {
							panic(fmt.Errorf("error reading from connection: %w", err))
						}
					}
					if n != 4 {
						panic(fmt.Errorf("expected to read 4 bytes, got %d", n))
					}
					// write it back
					n, err = conn.Write(serverBlob)
					if err != nil {
						if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
							slog.Debug("Connection is closed, shutting down accepted conn")
							return
						} else {
							panic(fmt.Errorf("error writing to connection: %w", err))
						}
					}
					if n != 4 {
						panic(fmt.Errorf("expected to write 4 bytes, got %d", n))
					}
				}
			}()
		}
	}()

	port := socket.Addr().(*net.TCPAddr).Port

	slog.Info("Listener port", slog.Int("port", port))

	clients := make([]*net.TCPConn, nGoRoutines)
	for i := 0; i < nGoRoutines; i++ {
		client, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
		if err != nil {
			t.Fatalf("error connecting to server: %v", err)
		}
		clients[i] = client.(*net.TCPConn)
		// set tcp no delay
		err = clients[i].SetNoDelay(tcpNoDelay)
		if err != nil {
			t.Fatalf("error setting no delay: %v", err)
		}
		// send window size
		err = clients[i].SetWriteBuffer(4)
		if err != nil {
			t.Fatalf("error setting write buffer: %v", err)
		}

		// set read buffer size
		err = clients[i].SetReadBuffer(4)
		if err != nil {
			t.Fatalf("error setting read buffer: %v", err)
		}
	}
	defer func() {
		for _, conn := range clients {
			if conn != nil {
				_ = conn.Close()
			}
		}
	}()

	for i := 0; i < 2; i++ {
		t0 := time.Now()
		totalWriteCount := atomic.Int64{}
		lop.ForEach(lo.Range(nGoRoutines), func(i int, _ int) {

			clientBlob := make([]byte, 4) // an int32

			clientSocket := clients[i]

			nReqs := int64(0)
			defer func() {
				totalWriteCount.Add(nReqs)
			}()
			for time.Since(t0) < testTime {
				// write
				n, err := clientSocket.Write(clientBlob)
				if err != nil {
					if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
						slog.Debug("Connection is closed, shutting down client")
						return
					} else {
						panic(fmt.Errorf("error writing to connection: %w", err))
					}
				}
				if n != 4 {
					panic(fmt.Errorf("expected to write 4 bytes, got %d", n))
				}
				// read
				n, err = clientSocket.Read(clientBlob)
				if err != nil {
					if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
						slog.Debug("Connection is closed, shutting down client")
						return
					} else {
						panic(fmt.Errorf("error reading from connection: %w", err))
					}
				}
				if n != 4 {
					panic(fmt.Errorf("expected to read 4 bytes, got %d", n))
				}
				nReqs++
			}
		})

		slog.Info("Test done")

		slog.Info(fmt.Sprintf("Total requests: %v", prettyInt3Digits(totalWriteCount.Load())))
	}

}
