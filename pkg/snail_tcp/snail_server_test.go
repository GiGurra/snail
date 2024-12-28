package snail_tcp

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"io"
	"log/slog"
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

	slog.Info("numTotalMessages", slog.Int("numTotalMessages", numTotalMessages))
	slog.Info("batchSize", slog.Int("batchSize", batchSize))
	slog.Info("nGoRoutines", slog.Int("nGoRoutines", nGoRoutines))
	slog.Info("nBatchesTotal", slog.Int("nBatchesTotal", nBatchesTotal))
	slog.Info("nBatchesPerRoutine", slog.Int("nBatchesPerRoutine", nBatchesPerRoutine))

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

	server, err := NewServer(newHandlerFunc, nil)
	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}
	defer server.Close()

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	slog.Info("Listener port", slog.Int("port", server.Port()))

	slog.Info("Sending all bytes")

	for i := 0; i < nGoRoutines; i++ {
		go func() {

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

	if atomicCounter.Load() != int64(numTotalMessages) {
		t.Fatalf("expected %d bytes, got %d", numTotalMessages, atomicCounter.Load())
	}

	slog.Info("Received all messages")

}
