package user

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"opencsg.com/csghub-server/builder/analytics"
	"opencsg.com/csghub-server/builder/instrumentation"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/builder/workhub"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/rebac/factory"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/user/router"
)

var cmdLaunch = &cobra.Command{
	Use:     "launch",
	Short:   "Launch user server",
	Example: serverExample(),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		slog.Debug("config", slog.Any("data", cfg))
		stopOtel, err := instrumentation.SetupOTelSDK(context.Background(), cfg, instrumentation.User)
		if err != nil {
			panic(err)
		}
		// Check APIToken length
		if len(cfg.APIToken) < 128 {
			return fmt.Errorf("API token length is less than 128, please check")
		}
		analyticsPublisher, analyticsErr := analytics.New(analytics.Config{
			Enabled:      cfg.PostHog.Enabled,
			ProjectToken: cfg.PostHog.ProjectToken,
			APIHost:      cfg.PostHog.APIHost,
			Environment:  cfg.PostHog.Environment,
		})
		if analyticsErr != nil {
			slog.Error("PostHog publisher is disabled", slog.Any("error", analyticsErr))
		}
		analytics.Assign(analyticsPublisher)
		defer func() {
			if closeErr := analyticsPublisher.Close(); closeErr != nil {
				slog.Warn("Failed to flush PostHog events during shutdown", slog.Any("error", closeErr))
			}
			analytics.Assign(nil)
		}()
		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(cfg.Database.Driver),
			DSN:     cfg.Database.DSN,
		}
		if err := database.InitDB(dbConfig); err != nil {
			slog.Error("failed to initialize database", slog.Any("error", err))
			return fmt.Errorf("database initialization failed: %w", err)
		}

		wfClient, err := temporal.NewClient(client.Options{
			HostPort: cfg.WorkFLow.Endpoint,
			Logger:   log.NewStructuredLogger(slog.Default()),
			ConnectionOptions: client.ConnectionOptions{
				GetSystemInfoTimeout: time.Duration(cfg.Temporal.GetSystemInfoTimeout) * time.Second,
			},
		}, "csghub-user")
		if err != nil {
			return fmt.Errorf("unable to create workflow client, error:%w", err)
		}

		userWorkClients, err := newUserWorkClients(cmd.Context(), cfg)
		if err != nil {
			return err
		}
		stopWorkers, err := startUserWorkClients(cmd.Context(), userWorkClients)
		if err != nil {
			return err
		}
		defer stopWorkers()

		r, err := router.NewRouter(cfg)
		if err != nil {
			return fmt.Errorf("failed to init router: %w", err)
		}
		slog.Info("http server is running", slog.Any("port", cfg.User.Port))
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.User.Port,
			},
			r,
		)
		server.Run()

		_ = stopOtel(context.Background())
		wfClient.Close()
		return nil
	},
}

func serverExample() string {
	return `
# for development
csghub-server user launch
`
}

// newUserWorkClients creates separate leased organization and tuple consumers.
func newUserWorkClients(ctx context.Context, cfg *config.Config) ([]workhub.WorkClient, error) {
	ssoClient, err := rpc.NewSSOClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create sso client: %w", err)
	}
	authorizer, err := factory.NewAuthorizer()
	if err != nil {
		return nil, fmt.Errorf("create rebac authorizer: %w", err)
	}
	organizationWorker := component.NewOrganizationDeletionWorker(ssoClient, database.NewRepositoryAuthorizationStore())
	organizationClient, err := component.NewOrganizationDeletionWorkClient(ctx, cfg.Database.DSN, organizationWorker)
	if err != nil {
		return nil, fmt.Errorf("create organization deletion work client: %w", err)
	}
	clients := []workhub.WorkClient{organizationClient}
	tupleClient, err := component.NewRebacTupleWorkClient(ctx, cfg.Database.DSN, component.NewRebacTupleWorker(authorizer))
	if err != nil {
		stopUserWorkClients(clients)
		return nil, fmt.Errorf("create ReBAC tuple work client: %w", err)
	}
	return append(clients, tupleClient), nil
}

// stopUserWorkClients gives each client an independent shutdown budget.
func stopUserWorkClients(clients []workhub.WorkClient) {
	for _, client := range clients {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := client.Stop(stopCtx); err != nil {
			slog.Error("failed to stop user work client", slog.Any("error", err))
		}
		cancel()
	}
}

// startUserWorkClients starts both consumers before HTTP and releases all clients on failure.
func startUserWorkClients(ctx context.Context, clients []workhub.WorkClient) (func(), error) {
	stopWorkers := func() { stopUserWorkClients(clients) }
	for _, client := range clients {
		if err := client.Start(ctx); err != nil {
			stopWorkers()
			return nil, fmt.Errorf("start user work client: %w", err)
		}
	}
	slog.Info("user workers started", slog.Any("queues", []string{workhub.OrganizationDeletionQueue, workhub.RebacTupleQueue}))
	return stopWorkers, nil
}
