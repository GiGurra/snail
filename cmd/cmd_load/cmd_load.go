package cmd_load

import (
	"github.com/GiGurra/boa/pkg/boa"
	"github.com/GiGurra/snail/cmd/cmd_load/cmd_load_h1"
	"github.com/spf13/cobra"
)

type Params struct {
}

func Cmd() *cobra.Command {
	params := Params{}
	return boa.Wrap{
		Use:         "load",
		Short:       "run load testing commands",
		Params:      &params,
		ParamEnrich: boa.ParamEnricherDefault,
		SubCommands: []*cobra.Command{
			cmd_load_h1.Cmd(),
		},
	}.ToCmd()
}
