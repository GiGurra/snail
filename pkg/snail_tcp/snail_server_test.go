package snail_tcp

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"io"
	"log/slog"
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

func TestNewServer_send10mBytes_oneByOne(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("json", "info", false)

	numTotalMessages := 10_000_000
	batchSize := 100_000
	nBatches := numTotalMessages / batchSize

	if //goland:noinspection GoBoolExpressions
	numTotalMessages%batchSize != 0 {
		t.Fatalf("numTotalMessages must be divisible by batchSize")
	}

	recvCh := make(chan byte, batchSize)
	batch := make([]byte, batchSize)

	newHandlerFunc := func() ServerConnHandler {
		return func(buffer *snail_buffer.Buffer, writer io.Writer) error {
			if buffer == nil || writer == nil {
				slog.Info("Closing connection")
				return nil
			} else {
				//slog.Info("Handler received data")
				for _, b := range buffer.ReadAll() {
					recvCh <- b
				}
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

	go func() {
		slog.Info("Sending all bytes")
		for i := 0; i < nBatches; i++ {
			err = client.SendBytes(batch)
			if err != nil {
				panic(fmt.Errorf("error sending msg: %w", err))
			}
		}
	}()

	nReceived := 0
	for nReceived < numTotalMessages {
		select {
		case _ = <-recvCh:
			nReceived++
		case <-time.After(1 * time.Second):
			t.Fatalf("timeout waiting for message")
			return
		}
	}

	if nReceived != numTotalMessages {
		t.Fatalf("expected %d bytes, got %d", numTotalMessages, nReceived)
	}

	slog.Info("Received all messages")

}
