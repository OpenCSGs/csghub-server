package deploy

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/instrumentation"
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

		stopOtel, err := instrumentation.SetupOTelSDK(context.Background(), config, instrumentation.Runner)
		if err != nil {
			panic(err)
		}
		s, cleanup, err := router.NewHttpServer(cmd.Context(), config)
		if err != nil {
			return fmt.Errorf("failed to create runner server: %w", err)
		}

		slog.Info("deploy runner is running", slog.Any("port", config.Space.RunnerServerPort))
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: config.Space.RunnerServerPort,
			},
			s,
		)
		// Register the SandboxV2 shutdown (closes its stopCh) so the informer and idle
		// sweeper goroutines exit within the graceful-shutdown window instead of dying
		// with the process. No-op for CE (cleanup == nil).
		if cleanup != nil {
			server.RegisterOnShutdown(cleanup)
		}
		server.Run()
		_ = stopOtel(context.Background())
		return nil
	},
}
