package cmd_load_h1

import (
	"fmt"
	"github.com/GiGurra/boa/pkg/boa"
	"github.com/spf13/cobra"
	"log/slog"
	"os"
)

type Params struct {
	Connections           boa.Required[int]    `help:"Number of connections to use"`
	MaxConcurrentRequests boa.Required[int]    `help:"Number of concurrent requests to use per connection"`
	Url                   boa.Required[string] `help:"Url to use" positional:"true"`
	Duration              boa.Optional[int]    `help:"Duration to run the test for in seconds"`
	NumberOfRequests      boa.Optional[int]    `help:"Number of requests to run"`
}

func Cmd() *cobra.Command {
	params := Params{}
	return boa.Wrap{
		Use:    "h1",
		Short:  "run http1.1 load testing",
		Params: &params,
		ParamEnrich: boa.ParamEnricherCombine(
			boa.ParamEnricherName,
			boa.ParamEnricherShort,
			boa.ParamEnricherBool,
		),
		Run: func(cmd *cobra.Command, args []string) {
			if params.Duration.HasValue() && params.NumberOfRequests.HasValue() {
				exitWithError("Cannot specify both duration and number of requests")
			}

			fmt.Printf("Will load test %s using:\n", params.Url.Value())
			fmt.Printf("  %d connection(s)\n", params.Connections.Value())
			fmt.Printf("  %d concurrent request(s) per connection\n", params.MaxConcurrentRequests.Value())
		},
	}.ToCmd()
}

func exitWithError(msg string) {
	slog.Error(msg)
	os.Exit(1)
}
