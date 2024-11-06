package trigger

import (
	"fmt"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component/callback"
)

func init() {
	Cmd.AddCommand(
		gitCallbackCmd,
		fixOrgDataCmd,
		fixUserDataCmd,
		updateRepoCmd,
	)
}

var Cmd = &cobra.Command{
	Use:   "trigger",
	Short: "trigger a specific command",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		config, err := config.LoadConfig()
		if err != nil {
			return
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		database.InitDB(dbConfig)
		if err != nil {
			err = fmt.Errorf("initializing DB connection: %w", err)
			return
		}
		rs = database.NewRepoStore()
		gs, err = git.NewGitServer(config)
		if err != nil {
			return
		}
		callbackComponent, err = callback.NewGitCallback(config)
		if err != nil {
			return
		}

		return
	},
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}
