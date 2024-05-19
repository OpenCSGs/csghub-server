package deploy

import (
	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

var startRunnerCmd = &cobra.Command{
	Use:   "runner",
	Short: "start space runner service",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		return
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := config.LoadConfig()
		if err != nil {
			return err
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}
		database.InitDB(dbConfig)

		s, err := imagerunner.NewHttpServer(config)
		if err != nil {
			return err
		}
		err = s.Run(8082)
		return err
	},
}
