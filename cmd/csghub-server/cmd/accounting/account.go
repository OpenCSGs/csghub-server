package accounting

import (
	"github.com/spf13/cobra"
)

func init() {
	// add subcommands here
	Cmd.AddCommand(launchCmd)
}

var Cmd = &cobra.Command{
	Use:   "accounting",
	Short: "entry point for accounting",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
