package cmd_load_h1

import (
	"github.com/GiGurra/boa/pkg/boa"
	"github.com/spf13/cobra"
	"log/slog"
)

type Params struct {
	Connections           boa.Required[int] `help:"Number of connections to use"`
	MaxConcurrentRequests boa.Required[int] `help:"Number of concurrent requests to use per connection"`
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
			slog.Info("Hello, World load h1!")
		},
	}.ToCmd()
}
