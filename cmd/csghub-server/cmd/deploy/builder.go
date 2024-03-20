package deploy

import (
	"github.com/spf13/cobra"
)

var startBuilderCmd = &cobra.Command{
	Use:   "builder",
	Short: "start space builder service",
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
