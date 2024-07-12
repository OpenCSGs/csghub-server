package syncversion

import (
	"github.com/spf13/cobra"
)

func init() {
	// add subcommands here
	Cmd.AddCommand(InitCmd)
}

var Cmd = &cobra.Command{
	Use:   "sync-version",
	Short: "entry point for mirror jobs",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
