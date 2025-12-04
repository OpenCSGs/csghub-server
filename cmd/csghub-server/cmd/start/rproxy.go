package start

import (
	"context"
	"fmt"
	"log/slog"
	"opencsg.com/csghub-server/builder/instrumentation"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/router"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

var rproxyCmd = &cobra.Command{
	Use:     "rproxy",
	Short:   "Start the reverse proxy server",
	Example: rproxyExample(),
	RunE: func(*cobra.Command, []string) (err error) {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		stopOtel, err := instrumentation.SetupOTelSDK(context.Background(), cfg, instrumentation.RProxy)
		if err != nil {
			panic(err)
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(cfg.Database.Driver),
			DSN:     cfg.Database.DSN,
		}
		if err := database.InitDB(dbConfig); err != nil {
			slog.Error("failed to initialize database", slog.Any("error", err))
			return fmt.Errorf("database initialization failed: %w", err)
		}
		r, err := router.NewRProxyRouter(cfg)
		if err != nil {
			return fmt.Errorf("failed to init router: %w", err)
		}
		slog.Info("rproxy http server is running", slog.Any("port", cfg.Space.RProxyServerPort))
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.Space.RProxyServerPort,
			},
			r,
		)
		server.Run()
		_ = stopOtel(context.Background())
		return nil
	},
}

func rproxyExample() string {
	return `
# for development
csghub-server start rproxy
`
}
