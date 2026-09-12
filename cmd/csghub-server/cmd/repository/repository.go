package repository

import "github.com/spf13/cobra"

func init() {
	Cmd.AddCommand(deletionWorkerCmd)
}

// Cmd groups repository maintenance processes.
var Cmd = &cobra.Command{
	Use:   "repository",
	Short: "entry point for repository background services",
	Run: func(cmd *cobra.Command, _ []string) {
		_ = cmd.Help()
	},
}
