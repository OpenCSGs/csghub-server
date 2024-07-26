package user

import "github.com/spf13/cobra"

func init() {
	// add subcommands here
	Cmd.AddCommand(cmdLaunch)
}

var Cmd = &cobra.Command{
	Use:   "user",
	Short: "entry point for user service",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
