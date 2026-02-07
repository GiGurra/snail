package cmd_load

import (
	"github.com/GiGurra/boa/pkg/boa"
	"github.com/GiGurra/snail/cmd/cmd_load/cmd_load_h1"
	"github.com/spf13/cobra"
)

type Params struct {
}

func Cmd() *cobra.Command {
	return boa.CmdT[Params]{
		Use:         "load",
		Short:       "run load testing commands",
		ParamEnrich: boa.ParamEnricherDefault,
		SubCmds: []*cobra.Command{
			cmd_load_h1.Cmd(),
		},
	}.ToCobra()
}
