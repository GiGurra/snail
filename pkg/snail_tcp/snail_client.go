package snail_tcp

import (
	"errors"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"log/slog"
	"net"
)

// ClientRespHandler is the custom response handler for a client connection.
type ClientRespHandler func(*snail_buffer.Buffer) error

type SnailClient struct {
	socket      net.Conn
	opts        SnailClientOpts
	respHandler ClientRespHandler
}

type OptimizationType int

const (
	OptimizeForLatency OptimizationType = iota
	OptimizeForThroughput
)

type SnailClientOpts struct {
	//MaxConnections int // TODO: implement support for this
	Optimization        OptimizationType
	ReadBufSize         int
	MaxBufferedRespData int // TODO: Make use of?
	WriteBufSize        int // TODO: Make use of?
	TcpSendWindowSize   int
	TcpReadWindowSize   int
}

func (o SnailClientOpts) WithDefaults() SnailClientOpts {
	res := o
	if res.ReadBufSize == 0 {
		res.ReadBufSize = 64 * 1024
	}
	if res.WriteBufSize == 0 {
		res.WriteBufSize = 64 * 1024
	}
	return res
}

func NewClient(
	ip string,
	port int,
	optsIn *SnailClientOpts,
	respHandler ClientRespHandler,
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
	if opts.TcpSendWindowSize > 0 {
		err = socket.(*net.TCPConn).SetWriteBuffer(opts.TcpSendWindowSize)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to set TCP send window size: %v. Proceeding anyway :S", err))
		}
	}
	if opts.TcpReadWindowSize > 0 {
		err = socket.(*net.TCPConn).SetReadBuffer(opts.TcpReadWindowSize)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to set TCP read window size: %v. Proceeding anyway :S", err))
		}
	}

	res := &SnailClient{
		socket:      socket,
		opts:        opts,
		respHandler: respHandler,
	}

	go res.loopRespListener()

	return res, nil
}

func (c *SnailClient) loopRespListener() {

	readBuffer := snail_buffer.New(snail_buffer.BigEndian, c.opts.ReadBufSize)

	for {

		// TODO: Respect c.opts.MaxBufferedRespData

		err := ReadToBuffer(c.opts.ReadBufSize/5, c.socket, readBuffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				slog.Debug("Client socket is closed, shutting down client")
				return
			} else {
				slog.Error(fmt.Sprintf("Failed to read bytes from socket, assuming connection broken, bailing: %v", err))
				return
			}
		}

		err = c.respHandler(readBuffer)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to handle response, assuming state broken, bailing: %v", err))
			return
		}
	}
}

func (c *SnailClient) Close() {
	err := c.socket.Close()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to close socket: %v", err))
	}
}

func (c *SnailClient) SendBytes(data []byte) error {
	return SendAll(c.socket, data)
}
