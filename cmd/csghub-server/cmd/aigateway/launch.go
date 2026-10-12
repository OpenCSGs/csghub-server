package aigateway

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"

	"opencsg.com/csghub-server/aigateway/component/upstream"
	"opencsg.com/csghub-server/aigateway/router"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/instrumentation"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
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
		if err := database.InitDB(dbConfig); err != nil {
			slog.Error("failed to initialize database", slog.Any("error", err))
			return fmt.Errorf("database initialization failed: %w", err)
		}
		stopOtel, err := instrumentation.SetupOTelSDK(context.Background(), cfg, instrumentation.Aigateway)
		if err != nil {
			panic(err)
		}
		allocationDispatchClient, err := temporal.NewLazyClient(client.Options{
			HostPort: cfg.WorkFLow.Endpoint,
			Logger:   log.NewStructuredLogger(slog.Default()),
			ConnectionOptions: client.ConnectionOptions{
				GetSystemInfoTimeout: time.Duration(cfg.Temporal.GetSystemInfoTimeout) * time.Second,
			},
		}, instrumentation.Aigateway)
		if err != nil {
			slog.Warn("agent allocation immediate dispatch is unavailable", slog.Any("error", err))
			allocationDispatchClient = nil
		}

		err = event.InitEventPublisher(cfg)
		if err != nil {
			return fmt.Errorf("fail to initialize message queue, %w", err)
		}

		// Start the deploy→upstream sync consumer.
		syncComp := upstream.NewAIGatewayUpstreamSyncComponentDefault(cfg)
		upstreamSyncConsumer := upstream.NewUpstreamSyncConsumer(event.DefaultEventPublisher.MQ, syncComp)
		if err := upstreamSyncConsumer.Start(); err != nil {
			slog.Error("failed to start upstream sync consumer", slog.Any("error", err))
		}

		r, cleanup, err := router.NewRouter(cfg)
		if err != nil {
			return fmt.Errorf("failed to init router: %w", err)
		}
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.AIGateway.Port,
			},
			r,
		)
		if cleanup != nil {
			server.RegisterOnShutdown(cleanup)
		}
		if allocationDispatchClient != nil {
			server.RegisterOnShutdown(allocationDispatchClient.Stop)
		}
		server.RegisterOnShutdown(upstreamSyncConsumer.Stop)
		slog.Info("http server is running", slog.Any("port", cfg.AIGateway.Port))
		server.Run()
		if cleanup != nil {
			cleanup()
		}
		_ = stopOtel(context.Background())
		return nil
	},
}

func serverExample() string {
	return `
# for development
csghub-server aigateway launch
`
}
