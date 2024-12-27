package custom_proto

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"log/slog"
	"testing"
	"time"
)

func TestBuildCustomProtoMsg(t *testing.T) {
	msg := BuildCustomProtoMsg(CstRequest, "12345678", Approved)

	if msg.RequestLength != 15 {
		t.Errorf("expected 15, got %d", msg.RequestLength)
	}

	t.Logf("msg: %+v", msg)
}

func TestWriteReadCustomProtoMsg(t *testing.T) {
	msg := BuildCustomProtoMsg(CstRequest, "12345678", Approved)

	buf := snail_buffer.New(snail_buffer.BigEndian, 1024)

	err := WriteMsg(buf, msg)
	if err != nil {
		t.Fatalf("error writing msg: %v", err)
	}

	t.Logf("buf: %v", buf)

	readMsgs, err := TryReadMsgs(buf)
	if err != nil {
		t.Fatalf("error reading msg: %v", err)
	}

	if len(readMsgs) != 1 {
		t.Fatalf("expected 1 msg, got %d", len(readMsgs))
	}

	if diff := cmp.Diff(msg, readMsgs[0]); diff != "" {
		t.Fatalf("msg mismatch (-want +got):\n%s", diff)
	}
}

func TestNewCustomProtoListener(t *testing.T) {

	snail_logging.ConfigureDefaultLogger("json", "info", false)

	server, err := NewCustomProtoServer(0, OptimizeForThroughput)
	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}
	defer server.Close()

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	slog.Info("Listener port", slog.Int("port", server.Port()))
	client, err := NewCustomProtoClient("localhost", server.Port(), OptimizeForThroughput)
	if err != nil {
		t.Fatalf("error creating client: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatalf("expected client, got nil")
	}

	msg := BuildCustomProtoMsg(CstRequest, "12345678", Pending)
	err = client.SendMsg(msg)
	if err != nil {
		t.Fatalf("error sending msg: %v", err)
	}

	slog.Info("Sent message")

	select {
	case recvMsgs := <-server.RecvCh():
		for _, recvMsg := range recvMsgs {
			if diff := cmp.Diff(msg, recvMsg); diff != "" {
				t.Fatalf("msg mismatch (-want +got):\n%s", diff)
			}
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timeout waiting for message")
	}

}

func TestBenchMarkServerClientSingleThread(t *testing.T) {

	nMessages := 1_000_000

	snail_logging.ConfigureDefaultLogger("text", "info", false)
	slog.Info("****************************************")
	slog.Info("Starting benchmark")

	server, err := NewCustomProtoServer(0, OptimizeForThroughput)
	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}
	defer server.Close()

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	slog.Info("Listener port", slog.Int("port", server.Port()))
	client, err := NewCustomProtoClient("localhost", server.Port(), OptimizeForThroughput)
	if err != nil {
		t.Fatalf("error creating client: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatalf("expected client, got nil")
	}

	t0 := time.Now()

	refMsg := BuildCustomProtoMsg(CstRequest, "12345678", Pending)
	errs := make(chan error, 1)
	go func() {
		for i := 0; i < nMessages; i++ {
			err = client.SendMsg(refMsg)
			if err != nil {
				slog.Error(fmt.Sprintf("error sending msg: %v", err))
				errs <- err
				return
			}
		}
	}()

	if len(errs) > 0 {
		t.Fatalf("error sending msg: %v", <-errs)
	}

	nMessagesReceived := 0
	for nMessagesReceived < nMessages {
		select {
		case recvMsgs := <-server.RecvCh():
			for range recvMsgs {
				//if diff := cmp.Diff(refMsg, recvMsg); diff != "" {
				//	t.Fatalf("msg mismatch (-want +got):\n%s", diff)
				//}
				nMessagesReceived++
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("timeout waiting for message")
		}
	}

	if nMessagesReceived != nMessages {
		t.Fatalf("expected %d messages, got %d", nMessages, nMessagesReceived)
	}

	elapsed := time.Since(t0)
	rate := float64(nMessages) / elapsed.Seconds()
	slog.Info(fmt.Sprintf("Elapsed time %s", elapsed))
	slog.Info(fmt.Sprintf("Message rate %s msg/s", fmtLargeIntIn3DigitSetsF(rate)))
	slog.Info("****************************************")

}

func TestBenchMarkServerClientSingleThreadBatch(t *testing.T) {

	nMessages := 1_000_000
	nBatchSize := 1_000

	snail_logging.ConfigureDefaultLogger("text", "info", false)
	slog.Info("****************************************")
	slog.Info("Starting benchmark")

	server, err := NewCustomProtoServer(0, OptimizeForThroughput)
	if err != nil {
		t.Fatalf("error creating server: %v", err)
	}
	defer server.Close()

	if server == nil {
		t.Fatalf("expected server, got nil")
	}

	slog.Info("Listener port", slog.Int("port", server.Port()))
	client, err := NewCustomProtoClient("localhost", server.Port(), OptimizeForThroughput)
	if err != nil {
		t.Fatalf("error creating client: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatalf("expected client, got nil")
	}

	t0 := time.Now()

	refMsg := BuildCustomProtoMsg(CstRequest, "12345678", Pending)
	errs := make(chan error, 1)
	go func() {
		batch := make([]CustomProtoMsg, 0, nBatchSize)
		for i := 0; i < nMessages; i++ {
			batch = append(batch, refMsg)
			if len(batch) == nBatchSize || i == nMessages-1 {
				//slog.Info(fmt.Sprintf("Sending batch of %d messages", nBatchSize))
				err = client.SendMsgs(batch)
				if err != nil {
					slog.Error(fmt.Sprintf("error sending msg: %v", err))
					errs <- err
					return
				}
				batch = batch[:0]
			}
		}
	}()

	if len(errs) > 0 {
		t.Fatalf("error sending msg: %v", <-errs)
	}

	nReceived := 0
	for nReceived < nMessages {
		select {
		case recvMsgs := <-server.RecvCh():
			for range recvMsgs {
				//if diff := cmp.Diff(refMsg, recvMsg); diff != "" {
				//	t.Fatalf("msg mismatch (-want +got):\n%s", diff)
				//}
				nReceived++
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timeout waiting for message")
		}
	}

	if nReceived != nMessages {
		t.Fatalf("expected %d messages, got %d", nMessages, nReceived)
	}

	elapsed := time.Since(t0)
	rate := float64(nMessages) / elapsed.Seconds()
	slog.Info(fmt.Sprintf("Elapsed time %s", elapsed))
	slog.Info(fmt.Sprintf("Message rate %s msg/s", fmtLargeIntIn3DigitSetsF(rate)))
	slog.Info("****************************************")

}

func TestBenchMarkServerClientMultiThread(t *testing.T) {

	nMessagesTotal := 10_000_000
	nGoRoutines := 10

	snail_logging.ConfigureDefaultLogger("text", "info", false)
	slog.Info("****************************************")
	slog.Info("Starting benchmark")

	t0 := time.Now()

	lop.ForEach(lo.Range(nGoRoutines), func(i int, _ int) {

		nMessages := nMessagesTotal / nGoRoutines

		slog.Info(fmt.Sprintf("Starting go routine %d, will send %d messages", i, nMessages))

		server, err := NewCustomProtoServer(0, OptimizeForThroughput)
		if err != nil {
			t.Fatalf("error creating server: %v", err)
		}
		defer server.Close()

		if server == nil {
			t.Fatalf("expected server, got nil")
		}

		slog.Info("Listener port", slog.Int("port", server.Port()))
		client, err := NewCustomProtoClient("localhost", server.Port(), OptimizeForThroughput)
		if err != nil {
			t.Fatalf("error creating client: %v", err)
		}
		defer client.Close()

		if client == nil {
			t.Fatalf("expected client, got nil")
		}

		refMsg := BuildCustomProtoMsg(CstRequest, "12345678", Pending)
		errs := make(chan error, 1)
		go func() {
			for i := 0; i < nMessages; i++ {
				err = client.SendMsg(refMsg)
				if err != nil {
					slog.Error(fmt.Sprintf("error sending msg: %v", err))
					errs <- err
					return
				}
			}
		}()

		if len(errs) > 0 {
			t.Fatalf("error sending msg: %v", <-errs)
		}

		nMessagesReceived := 0
		for nMessagesReceived < nMessages {
			select {
			case recvMsgs := <-server.RecvCh():
				for range recvMsgs {
					//if diff := cmp.Diff(refMsg, recvMsg); diff != "" {
					//	t.Fatalf("msg mismatch (-want +got):\n%s", diff)
					//}
					nMessagesReceived++
				}
			case <-time.After(1 * time.Second):
				t.Fatalf("timeout waiting for message")
			}
		}

		if nMessagesReceived != nMessages {
			t.Fatalf("expected %d messages, got %d", nMessages, nMessagesReceived)
		}

	})

	elapsed := time.Since(t0)
	rate := float64(nMessagesTotal) / elapsed.Seconds()
	slog.Info(fmt.Sprintf("Elapsed time %s", elapsed))
	slog.Info(fmt.Sprintf("Message rate %s msg/s", fmtLargeIntIn3DigitSetsF(rate)))
	slog.Info("****************************************")

}

func TestBenchMarkServerClientMultiThreadBatch(t *testing.T) {

	nMessagesTotal := 10_000_000
	nBatchSize := 1_000
	nGoRoutines := 10

	snail_logging.ConfigureDefaultLogger("text", "info", false)
	slog.Info("****************************************")
	slog.Info("Starting benchmark")

	t0 := time.Now()

	lop.ForEach(lo.Range(nGoRoutines), func(i int, _ int) {

		nMessages := nMessagesTotal / nGoRoutines

		slog.Info(fmt.Sprintf("Starting go routine %d, will send %d messages", i, nMessages))

		server, err := NewCustomProtoServer(0, OptimizeForThroughput)
		if err != nil {
			t.Fatalf("error creating server: %v", err)
		}
		defer server.Close()

		if server == nil {
			t.Fatalf("expected server, got nil")
		}

		slog.Info("Listener port", slog.Int("port", server.Port()))
		client, err := NewCustomProtoClient("localhost", server.Port(), OptimizeForThroughput)
		if err != nil {
			t.Fatalf("error creating client: %v", err)
		}
		defer client.Close()

		if client == nil {
			t.Fatalf("expected client, got nil")
		}

		refMsg := BuildCustomProtoMsg(CstRequest, "12345678", Pending)
		errs := make(chan error, 1)
		go func() {
			batch := make([]CustomProtoMsg, 0, nBatchSize)
			for i := 0; i < nMessages; i++ {
				batch = append(batch, refMsg)
				if len(batch) == nBatchSize || i == nMessages-1 {
					//slog.Info(fmt.Sprintf("Sending batch of %d messages", nBatchSize))
					err = client.SendMsgs(batch)
					if err != nil {
						slog.Error(fmt.Sprintf("error sending msg: %v", err))
						errs <- err
						return
					}
					batch = batch[:0]
				}
			}
		}()

		if len(errs) > 0 {
			t.Fatalf("error sending msg: %v", <-errs)
		}

		nMessagesReceived := 0
		for nMessagesReceived < nMessages {
			select {
			case recvMsgs, chOpen := <-server.RecvCh():
				if !chOpen {
					t.Fatalf("server recv channel closed")
				}
				for range recvMsgs {
					//if diff := cmp.Diff(refMsg, recvMsg); diff != "" {
					//	t.Fatalf("msg mismatch (-want +got):\n%s", diff)
					//}
					nMessagesReceived++
				}
			case <-time.After(1 * time.Second):
				t.Fatalf("timeout waiting for message")
			}
		}

		if nMessagesReceived != nMessages {
			t.Fatalf("expected %d messages, got %d", nMessages, nMessagesReceived)
		}

	})

	elapsed := time.Since(t0)
	rate := float64(nMessagesTotal) / elapsed.Seconds()
	slog.Info(fmt.Sprintf("Elapsed time %s", elapsed))
	slog.Info(fmt.Sprintf("Message rate %s msg/s", fmtLargeIntIn3DigitSetsF(rate)))
	slog.Info("****************************************")

}

func fmtLargeIntIn3DigitSets(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) < 4 {
		return s
	}
	return fmtLargeIntIn3DigitSets(n/1_000) + "," + s[len(s)-3:]
}

func fmtLargeIntIn3DigitSetsF(n float64) string {
	return fmtLargeIntIn3DigitSets(int64(n))
}
