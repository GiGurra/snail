package cmd_load_h1

import (
	"fmt"
	"github.com/GiGurra/boa/pkg/boa"
	"github.com/GiGurra/snail/pkg/snail_batcher"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/GiGurra/snail/pkg/snail_tcp"
	"github.com/GiGurra/snail/pkg/snail_test_util/strutil"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"github.com/spf13/cobra"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Params struct {
	Connections      boa.Required[int]    `descr:"Number of connections to use"`
	Url              boa.Required[string] `descr:"Url to use" positional:"true"`
	BatchSizeBytes   boa.Required[int]    `descr:"Batch size in bytes for tcp writes" default:"65536"`
	Duration         boa.Optional[string] `descr:"Duration to run the test for"`
	NumberOfRequests boa.Optional[int]    `descr:"Number of requests to run"`
}

func (p *Params) WithValidation() *Params {
	p.Connections.CustomValidator = func(i int) error {
		if i <= 0 {
			return fmt.Errorf("connections must be greater than 0")
		}
		return nil
	}
	p.Url.CustomValidator = func(s string) error {
		// try parse as url
		_, err := url.Parse(s)
		if err != nil {
			return fmt.Errorf("url is not valid: %v", err)
		}
		return nil
	}
	p.Duration.CustomValidator = func(durationStr string) error {
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			return fmt.Errorf("duration is not valid: %v", err)
		} else if d.Seconds() < 1.0 {
			return fmt.Errorf("duration must be at least 1 second")
		}
		return nil
	}
	p.NumberOfRequests.CustomValidator = func(i int) error {
		if i <= 0 {
			return fmt.Errorf("number of requests must be greater than 0")
		}
		return nil
	}
	p.BatchSizeBytes.CustomValidator = func(i int) error {
		if i <= 0 {
			return fmt.Errorf("batch size must be greater than 0")
		}
		return nil
	}
	return p
}

func Cmd() *cobra.Command {
	params := new(Params).WithValidation()
	return boa.Wrap{
		Use:    "h1",
		Short:  "run http1.1 load testing",
		Params: params,
		ParamEnrich: boa.ParamEnricherCombine(
			boa.ParamEnricherName,
			boa.ParamEnricherShort,
			boa.ParamEnricherBool,
		),
		Run: func(cmd *cobra.Command, args []string) {
			if params.Duration.HasValue() && params.NumberOfRequests.HasValue() {
				exitWithError("Cannot specify both duration and number of requests")
			}

			if !params.Duration.HasValue() && !params.NumberOfRequests.HasValue() {
				exitWithError("Must specify either duration or number of requests")
			}

			fmt.Printf("* Will load test %s using:\n", params.Url.Value())
			fmt.Printf("  %d connection(s)\n", params.Connections.Value())
			fmt.Printf("  %d batch size (bytes)\n", params.BatchSizeBytes.Value())
			if params.Duration.HasValue() {
				duration, _ := time.ParseDuration(*params.Duration.Value())
				fmt.Printf("  %s test duration\n", duration)
			} else if params.NumberOfRequests.HasValue() {
				fmt.Printf("  %d request(s) total spread across connections\n", *params.NumberOfRequests.Value())

				// Number of requests must be divisible by number of connections
				if *params.NumberOfRequests.Value()%params.Connections.Value() != 0 {
					exitWithError("Number of requests must be divisible by number of connections")
				}
			}

			parsedUrl, _ := url.Parse(params.Url.Value())
			host := parsedUrl.Hostname()
			port := func() int {
				if parsedUrl.Port() == "" {
					return 80
				}
				port, err := strconv.Atoi(parsedUrl.Port())
				if err != nil {
					exitWithError(fmt.Sprintf("Failed to parse port: %v", err))
				}
				return port
			}()

			fmt.Println("* Creating clients")
			clients := make([]*snail_tcp.SnailClient, params.Connections.Value())
			respHandlers := make([]snail_tcp.ClientRespHandler, params.Connections.Value())
			for i := 0; i < params.Connections.Value(); i++ {
				var err error
				clients[i], err = snail_tcp.NewClient(host, port, nil, func(buffer *snail_buffer.Buffer) error {
					return respHandlers[i](buffer)
				})
				if err != nil {
					exitWithError(fmt.Sprintf("Failed to create client: %v", err))
				}

				//goland:noinspection GoDeferInLoop // it's intended
				defer clients[i].Close()
			}

			fmt.Println("* Creating batchers")
			batchers := make([]*snail_batcher.SnailBatcher[byte], params.Connections.Value())
			for i := 0; i < params.Connections.Value(); i++ {
				client := clients[i]
				batchers[i] = snail_batcher.NewSnailBatcher(
					params.BatchSizeBytes.Value(),
					params.BatchSizeBytes.Value()*2,
					false,
					1*time.Minute,
					func(bytes []byte) error {
						err := client.SendBytes(bytes)
						if err != nil {
							return fmt.Errorf("failed to send bytes: %v", err)
						}
						return nil
					},
				)
				//goland:noinspection GoDeferInLoop // it's intended
				defer batchers[i].Close()
			}

			type Status int
			const (
				LookingForStart Status = iota
				LookingForEnd
			)
			type receiveState struct {
				Status        Status
				StartAt       int
				EndAt         int
				ConsecutiveLF int
			}

			// If in fixed number of requests mode, we need to calculate how many requests each client should send
			if params.NumberOfRequests.HasValue() {
				requestsPerClient := *params.NumberOfRequests.Value() / params.Connections.Value()
				lop.ForEach(lo.Range(params.Connections.Value()), func(i int, _ int) {

					responsesReceived := 0
					doneChan := make(chan struct{})

					respHandlers[i] = func(buffer *snail_buffer.Buffer) error {
						bytes := buffer.Underlying()
						start := buffer.ReadPos()
						ln := len(bytes)

						state := receiveState{}

						for i := start; i < ln; i++ {
							b := bytes[i]
							if b == '\r' {
								continue // ignore
							}
							switch state.Status {
							case LookingForStart:
								if b == 'H' {
									// found the start!
									state.StartAt = i
									state.Status = LookingForEnd
								}
							case LookingForEnd:
								if b == '\n' {
									state.ConsecutiveLF++
									if state.ConsecutiveLF == 2 {
										responsesReceived++
										buffer.SetReadPos(i + 1)
										state = receiveState{}
									}
								} else {
									state.ConsecutiveLF = 0
								}
							}
						}
						buffer.DiscardReadBytes()

						if responsesReceived == requestsPerClient {
							close(doneChan)
						}

						return nil
					}

					batcher := batchers[i]
					for j := 0; j < requestsPerClient; j++ {
						batcher.AddMany(defaultRequestString)
					}
					batcher.Flush()

					<-doneChan
				})

			} else if params.Duration.HasValue() {

			} else {
				exitWithError("Invalid application mode. Neither duration or number of requests was set")
			}
		},
	}.ToCmd()
}

func exitWithError(msg string) {
	slog.Error(msg)
	os.Exit(1)
}

var defaultRequestString = []byte(strutil.StripMargin(
	`|GET / HTTP/1.1
		 |Host: localhost:9080
		 |User-Agent: snail
		 |Accept: */*
		 |
		 |`,
))
