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
	boa.CmdT[Params]{
		Use:         "snail",
		Short:       "run snail tools",
		ParamEnrich: boa.ParamEnricherDefault,
		SubCmds: []*cobra.Command{
			cmd_load.Cmd(),
		},
	}.Run()
}
