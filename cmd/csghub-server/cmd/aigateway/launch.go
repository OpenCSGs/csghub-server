package aigateway

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/aigateway/router"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

var cmdLaunch = &cobra.Command{
	Use:     "launch",
	Short:   "Launch aigateway server",
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
		err = event.InitEventPublisher(cfg, nil)
		if err != nil {
			return fmt.Errorf("fail to initialize message queue, %w", err)
		}

		r, err := router.NewRouter(cfg)
		if err != nil {
			return fmt.Errorf("failed to init router: %w", err)
		}
		slog.Info("http server is running", slog.Any("port", cfg.AIGateway.Port))
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.AIGateway.Port,
			},
			r,
		)
		server.Run()

		return nil
	},
}

func serverExample() string {
	return `
# for development
csghub-server aigateway launch
`
}
