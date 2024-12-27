package custom_proto

import (
	"fmt"
	"log/slog"
	"net"
)

type SnailClient struct {
	socket net.Conn
	opts   SnailClientOpts
}

type OptimizationType int

const (
	OptimizeForLatency OptimizationType = iota
	OptimizeForThroughput
)

type SnailClientOpts struct {
	//MaxConnections int // TODO: implement support for this
	Optimization OptimizationType
	ReadBufSize  int // TODO: Make use of?
	WriteBufSize int // TODO: Make use of?
}

func (o SnailClientOpts) WithDefaults() SnailClientOpts {
	res := o
	if res.ReadBufSize == 0 {
		res.ReadBufSize = 2048
	}
	if res.WriteBufSize == 0 {
		res.WriteBufSize = 2048
	}
	return res
}

func NewClient(
	ip string,
	port int,
	optsIn *SnailClientOpts,
) (*SnailClient, error) {
	socket, err := net.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))
	opts := func() SnailClientOpts {
		if optsIn == nil {
			return SnailClientOpts{}
		}
		return *optsIn
	}().WithDefaults()
	if err != nil {
		return nil, err
	}
	if opts.Optimization == OptimizeForThroughput {
		err = socket.(*net.TCPConn).SetNoDelay(false) // we favor throughput over latency here. This gets us massive speedups. (300k msgs/s -> 1m)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to set TCP_NODELAY=false: %v. Proceeding anyway :S", err))
		}
	} else {
		err = socket.(*net.TCPConn).SetNoDelay(true) // we favor latency over throughput here.
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to set TCP_NODELAY=true: %v. Proceeding anyway :S", err))
		}
	}
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to set TCP_NODELAY: %v. Proceeding anyway :S", err))
	}
	return &SnailClient{
		socket: socket,
		opts:   opts,
	}, nil
}

func (c *SnailClient) Close() {
	err := c.socket.Close()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to close socket: %v", err))
	}
}

func (c *SnailClient) SendBytes(data []byte) error {

	nWritten := 0
	for nWritten < len(data) {
		n, err := c.socket.Write(data[nWritten:])
		if err != nil {
			return fmt.Errorf("failed to write msg to socket: %w", err)
		}
		if n < 0 {
			return fmt.Errorf("failed to write msg to socket, n < 0")
		}
		nWritten += n
	}

	return nil
}
