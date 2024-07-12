package mirror

import (
	"github.com/spf13/cobra"
)

func init() {
	// add subcommands here
	Cmd.AddCommand(createMirrorRepoFromFile)
	Cmd.AddCommand(checkMirrorProgress)
}

var Cmd = &cobra.Command{
	Use:   "mirror",
	Short: "entry point for mirror jobs",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
