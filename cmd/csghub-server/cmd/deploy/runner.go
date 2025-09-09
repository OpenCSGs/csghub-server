package deploy

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
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

		s, err := router.NewHttpServer(cmd.Context(), config)
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
		server.Run()
		return nil
	},
}
