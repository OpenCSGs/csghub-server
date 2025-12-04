package dataviewer

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/instrumentation"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/dataviewer/router"
)

var launchCmd = &cobra.Command{
	Use:     "launch",
	Short:   "Launch data viewer server",
	Example: serverExample(),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		slog.Debug("config", slog.Any("data", cfg))

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

		stopOtel, err := instrumentation.SetupOTelSDK(context.Background(), cfg, instrumentation.Dataviewer)
		if err != nil {
			panic(err)
		}

		client, err := temporal.NewClient(client.Options{
			HostPort: cfg.WorkFLow.Endpoint,
			Logger:   log.NewStructuredLogger(slog.Default()),
		}, "dataset-viewer")
		if err != nil {
			return fmt.Errorf("unable to create workflow client, error: %w", err)
		}
		r, err := router.NewDataViewerRouter(cfg, client)
		if err != nil {
			return fmt.Errorf("failed to init dataviewer router: %w", err)
		}

		slog.Info("dataviewer http server is running", slog.Any("port", cfg.DataViewer.Port))
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.DataViewer.Port,
			},
			r,
		)
		server.Run()
		_ = stopOtel(context.Background())

		temporal.Stop()

		return nil
	},
}

func serverExample() string {
	return `
# for development
csghub-server dataviewer launch
`
}
