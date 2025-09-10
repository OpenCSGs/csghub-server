//go:build saas

package trigger

func addCommands() {
	Cmd.AddCommand(
		fixRepoDescriptionCmd,
		fixMCPServerAvatarCmd,
		fixMCPServerLicenseTagCmd,
		fixLfsObjectPathCmd,
	)
}
