package cron

import (
	"github.com/spf13/cobra"
)

func init() {
	// add subcommands here
	Cmd.AddCommand(cmdCalcRecomScore)
	Cmd.AddCommand(cmdCreatePushMirror)
	Cmd.AddCommand(cmdSyncAsClient)
	Cmd.AddCommand(cmdGenTelemetry)
}

var Cmd = &cobra.Command{
	Use:   "cron",
	Short: "entry point for cron jobs",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
