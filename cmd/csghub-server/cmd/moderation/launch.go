package moderation

import (
	"context"
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"opencsg.com/csghub-server/builder/instrumentation"
	"opencsg.com/csghub-server/builder/temporal"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/moderation/checker"
	"opencsg.com/csghub-server/moderation/router"
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
		stopOtel, err := instrumentation.SetupOTelSDK(context.Background(), cfg, instrumentation.Moderation)
		if err != nil {
			panic(err)
		}
		// Check APIToken length
		if len(cfg.APIToken) < 128 {
			return fmt.Errorf("API token length is less than 128, please check")
		}
		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(cfg.Database.Driver),
			DSN:     cfg.Database.DSN,
		}
		if err := database.InitDB(dbConfig); err != nil {
			slog.Error("failed to initialize database", slog.Any("error", err))
			return fmt.Errorf("database initialization failed: %w", err)
		}
		checker.Init(cfg)
		slog.Info("starting temporal client")
		temporalClient, err := temporal.NewClient(client.Options{
			HostPort: cfg.WorkFLow.Endpoint,
			Logger:   log.NewStructuredLogger(slog.Default()),
		}, instrumentation.Moderation)
		if err != nil {
			return fmt.Errorf("unable to create temporal client, error: %w", err)
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

		_ = stopOtel(context.Background())
		temporalClient.Close()
		return nil
	},
}

func serverExample() string {
	return `
# for development
csghub-server moderation launch
`
}
