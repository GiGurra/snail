package main

import (
	"errors"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_tcp"
	"github.com/GiGurra/snail/pkg/snail_tcp/manual_perf_tests/common"
	"github.com/GiGurra/snail/pkg/snail_test_util"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	// This test is a reference test the golang tcp performance.
	testTime := 1 * time.Second

	// Create a server socket
	slog.Info("Connecting to port", slog.Int("port", common.Port))

	// create clients
	clients := make([]net.Conn, common.NClients)
	writeBufs := make([][]byte, common.NClients)
	for i := 0; i < common.NClients; i++ {
		writeBufs[i] = make([]byte, common.WriteBufSize)
		var err error
		clients[i], err = net.Dial("tcp", fmt.Sprintf("%s:%d", "localhost", common.Port))
		if err != nil {
			panic(fmt.Sprintf("error connecting to server: %v", err))
		}
		if //goland:noinspection GoBoolExpressions
		common.Optimization == snail_tcp.OptimizeForThroughput {
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
		if common.TcpWindowSize > 0 {
			err = clients[i].(*net.TCPConn).SetWriteBuffer(common.TcpWindowSize)
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

	keepRunning := snail_test_util.TrueForTimeout(testTime)

	// Run all writers
	slog.Info("Starting test")
	t0 := time.Now()
	wgWrite := sync.WaitGroup{}
	totalWrittenBytes := atomic.Int64{}
	for i, client := range clients {
		wgWrite.Add(1)
		go func(writeBuf []byte) {

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
		}(writeBufs[i])
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

	slog.Info("Test done")

	slog.Info(fmt.Sprintf("Total bytes written: %v", snail_test_util.PrettyInt3Digits(totalWrittenBytes.Load())))

	byteRate := float64(totalWrittenBytes.Load()) / elapsed.Seconds()
	bitRate := byteRate * 8

	slog.Info(fmt.Sprintf("Byte rate: %v", snail_test_util.PrettyInt3Digits(int64(byteRate))))
	slog.Info(fmt.Sprintf("Bit rate: %v", snail_test_util.PrettyInt3Digits(int64(bitRate))))
}
