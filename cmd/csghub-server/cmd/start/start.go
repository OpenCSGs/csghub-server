package start

import (
	"github.com/spf13/cobra"
)

func init() {
	Cmd.AddCommand(serverCmd)
	Cmd.AddCommand(rproxyCmd)
}

var Cmd = &cobra.Command{
	Use:   "start",
	Short: "Start a service",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}
