package snail_tcp_reqrep

import (
	"encoding/json"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"github.com/GiGurra/snail/pkg/snail_parser"
	"log/slog"
	"net"
	"testing"
)

type requestStruct struct {
	Msg string
}

type responseStruct struct {
	Msg string
}

func TestNewServer_SendAndRespondWithJson(t *testing.T) {
	snail_logging.ConfigureDefaultLogger("text", "info", false)

	slog.Info("TestNewServer_SendAndRespondWithJson")

	reqCodec := snail_parser.NewJsonLinesCodec[requestStruct]()
	respCodec := snail_parser.NewJsonLinesCodec[responseStruct]()

	server, err := NewServer[requestStruct, responseStruct](
		func() ServerConnHandler[requestStruct, responseStruct] {
			return func(req *requestStruct, repFunc func(resp *responseStruct) error) error {

				if req == nil || repFunc == nil {
					slog.Warn("Client disconnected")
					return nil
				}

				slog.Info("Server received request", slog.String("msg", req.Msg))
				return repFunc(&responseStruct{Msg: "Hello from server"})
			}
		},
		nil,
		reqCodec.Parser,
		respCodec.Writer,
	)

	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	defer server.Close()

	// Open a socket to the server and send a request
	sock, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", server.Underlying().Port()))
	if err != nil {
		t.Fatalf("error dialing server: %v", err)
	}

	// Send a request
	_, err = sock.Write([]byte(`{"Msg":"Hello from client"}` + "\n"))
	if err != nil {
		t.Fatalf("error sending request: %v", err)
	}

	// Read the response
	buf := make([]byte, 1024)
	n, err := sock.Read(buf)
	if err != nil {
		t.Fatalf("error reading response: %v", err)
	}

	resp := responseStruct{}
	err = json.Unmarshal(buf[:n], &resp)
	if err != nil {
		t.Fatalf("error unmarshalling response: %v", err)
	}

	if resp.Msg != "Hello from server" {
		t.Fatalf("expected response 'Hello from server', got '%s'", resp.Msg)
	}

	slog.Info("Received response", slog.String("msg", resp.Msg))
}
