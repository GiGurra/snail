package custom_proto

import (
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestNewServer_sendMessageTpServer(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("json", "info", true)

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

	server, err := NewServer(0, newHandlerFunc, nil)
	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}
	defer server.Close()

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	slog.Info("Listener port", slog.Int("port", server.Port()))
	client, err := NewClient("localhost", server.Port(), nil)
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
