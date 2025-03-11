package deploy

import (
	"fmt"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/runner/router"
)

var startRunnerCmd = &cobra.Command{
	Use:   "runner",
	Short: "start runner service",
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
		err = event.InitEventPublisher(config, nil)
		if err != nil {
			return fmt.Errorf("fail to initialize message queue, %w", err)
		}
		s, err := router.NewHttpServer(config)
		if err != nil {
			return err
		}
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: config.Space.RunnerServerPort,
			},
			s,
		)
		server.Run()
		return nil
	},
}
