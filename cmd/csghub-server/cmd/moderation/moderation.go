package moderation

import "github.com/spf13/cobra"

func init() {
	// add subcommands here
	Cmd.AddCommand(cmdLaunch)
}

var Cmd = &cobra.Command{
	Use:   "moderation",
	Short: "entry point for moderation service",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
