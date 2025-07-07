package notification

import (
	"github.com/spf13/cobra"
)

func init() {
	// add subcommands here
	Cmd.AddCommand(launchCmd)
}

var Cmd = &cobra.Command{
	Use:   "notification",
	Short: "entry point for notification server",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
