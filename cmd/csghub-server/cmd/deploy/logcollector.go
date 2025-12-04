package deploy

import (
	"context"
	"github.com/spf13/cobra"
	"log/slog"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/instrumentation"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/logcollector/router"
)

var logCollectorCmd = &cobra.Command{
	Use:   "logcollector",
	Short: "start logcollector service",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		stopOtel, err := instrumentation.SetupOTelSDK(context.Background(), cfg, instrumentation.Logcollector)
		if err != nil {
			panic(err)
		}
		s, logFactory, err := router.NewHttpServer(cmd.Context(), cfg)
		if err != nil {
			return err
		}

		slog.Info("deploy logcollector is running", slog.Any("port", cfg.LogCollector.Port))
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.LogCollector.Port,
			},
			s,
		)
		server.Run()
		logFactory.Stop()
		_ = stopOtel(context.Background())
		return nil
	},
}
