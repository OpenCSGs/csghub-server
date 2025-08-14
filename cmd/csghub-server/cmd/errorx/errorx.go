package errorx

import "github.com/spf13/cobra"

func init() {
	Cmd.AddCommand(docGenCmd)
}

var Cmd = &cobra.Command{
	Use:   "errorx",
	Short: "error code related commands",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
