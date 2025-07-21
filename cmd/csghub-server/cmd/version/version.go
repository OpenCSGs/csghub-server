package version

import (
	"github.com/spf13/cobra"
	v "opencsg.com/csghub-server/version"
)

var Cmd = &cobra.Command{
	Use: "version",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("git: %s\n", v.GitRevision)
		cmd.Printf("version: %s\n", v.StarhubAPIVersion)
	},
}
