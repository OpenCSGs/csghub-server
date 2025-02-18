package git

import (
	"github.com/spf13/cobra"
)

func init() {
	Cmd.AddCommand(generateLfsMetaObjectsCmd)
	Cmd.AddCommand(cloneProjectStorageCmd)
}

var Cmd = &cobra.Command{
	Use:   "git",
	Short: "git related commands",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
