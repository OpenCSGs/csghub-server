package temporal_worker

import (
	"github.com/spf13/cobra"
)

func init() {
	// add subcommands here
	Cmd.AddCommand(cmdLaunch)
}

var Cmd = &cobra.Command{
	Use:   "temporal-worker",
	Short: "entry point for temporal-worker service",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
