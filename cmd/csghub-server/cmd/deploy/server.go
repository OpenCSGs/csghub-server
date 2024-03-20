package deploy

import (
	"github.com/spf13/cobra"
)

func init() {
	Cmd.AddCommand(
		startBuilderCmd,
		startRunnerCmd,
	)
}

var Cmd = &cobra.Command{
	Use:   "deploy",
	Short: "entry point of space builder and runner",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		// config, err := config.LoadConfig()
		// if err != nil {
		// 	return
		// }

		return
	},
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
