package sync

import (
	"github.com/spf13/cobra"
)

func init() {
	// add subcommands here
	Cmd.AddCommand(cmdSyncAsClient)
}

var Cmd = &cobra.Command{
	Use:   "sync",
	Short: "entry point for mirror jobs",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
