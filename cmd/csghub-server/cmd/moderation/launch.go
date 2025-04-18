package moderation

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/moderation/checker"
	"opencsg.com/csghub-server/moderation/router"
	"opencsg.com/csghub-server/moderation/workflow"
)

var cmdLaunch = &cobra.Command{
	Use:     "launch",
	Short:   "Launch moderation server",
	Example: serverExample(),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		slog.Debug("config", slog.Any("data", cfg))
		// Check APIToken length
		if len(cfg.APIToken) < 128 {
			return fmt.Errorf("API token length is less than 128, please check")
		}
		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(cfg.Database.Driver),
			DSN:     cfg.Database.DSN,
		}
		database.InitDB(dbConfig)
		checker.Init(cfg)

		//init async moderation process
		err = workflow.StartWorker(cfg)
		if err != nil {
			return fmt.Errorf("failed to start workflow worker,%w", err)
		}

		r, err := router.NewRouter(cfg)
		if err != nil {
			return fmt.Errorf("failed to init router: %w", err)
		}
		slog.Info("moderation http server is running", slog.Any("port", cfg.Moderation.Port))
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.Moderation.Port,
			},
			r,
		)
		server.Run()

		workflow.StopWorker()

		return nil
	},
}

func serverExample() string {
	return `
# for development
csghub-server moderation launch
`
}
