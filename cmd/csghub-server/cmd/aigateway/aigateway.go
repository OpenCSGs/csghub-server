package aigateway

import "github.com/spf13/cobra"

func init() {
	// add subcommands here
	Cmd.AddCommand(cmdLaunch)
}

var Cmd = &cobra.Command{
	Use:   "aigateway",
	Short: "entry point for aigateway service",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
