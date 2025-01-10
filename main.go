package main

import (
	"github.com/GiGurra/boa/pkg/boa"
	"github.com/GiGurra/snail/cmd/cmd_load"
	"github.com/GiGurra/snail/pkg/snail_logging"
	"github.com/spf13/cobra"
)

type Params struct {
}

func main() {
	snail_logging.ConfigureDefaultLogger("text", "info", false)
	params := Params{}
	boa.Wrap{
		Use:         "snail",
		Short:       "run snail tools",
		Params:      &params,
		ParamEnrich: boa.ParamEnricherDefault,
		SubCommands: []*cobra.Command{
			cmd_load.Cmd(),
		},
	}.ToApp()
}
