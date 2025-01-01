package cmd_load_h1

import (
	"fmt"
	"github.com/GiGurra/boa/pkg/boa"
	"github.com/spf13/cobra"
	"log/slog"
	"net/url"
	"os"
	"time"
)

type Params struct {
	Connections           boa.Required[int]    `help:"Number of connections to use"`
	MaxConcurrentRequests boa.Required[int]    `help:"Number of concurrent requests to use per connection"`
	Url                   boa.Required[string] `help:"Url to use" positional:"true"`
	Duration              boa.Optional[string] `help:"Duration to run the test for"`
	NumberOfRequests      boa.Optional[int]    `help:"Number of requests to run"`
}

func (p *Params) WithValidation() *Params {
	p.Connections.CustomValidator = func(i int) error {
		if i <= 0 {
			return fmt.Errorf("connections must be greater than 0")
		}
		return nil
	}
	p.MaxConcurrentRequests.CustomValidator = func(i int) error {
		if i <= 0 {
			return fmt.Errorf("max concurrent requests must be greater than 0")
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

			fmt.Printf("Will load test %s using:\n", params.Url.Value())
			fmt.Printf("  %d connection(s)\n", params.Connections.Value())
			fmt.Printf("  %d concurrent request(s) per connection\n", params.MaxConcurrentRequests.Value())
			if params.Duration.HasValue() {
				duration, _ := time.ParseDuration(*params.Duration.Value())
				fmt.Printf("  %s test duration\n", duration)
			} else if params.NumberOfRequests.HasValue() {
				fmt.Printf("  %d request(s) total spread across connections\n", *params.NumberOfRequests.Value())
			}
		},
	}.ToCmd()
}

func exitWithError(msg string) {
	slog.Error(msg)
	os.Exit(1)
}
