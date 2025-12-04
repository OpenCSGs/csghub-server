package sync

import (
	"context"
	"fmt"
	"log/slog"
	"opencsg.com/csghub-server/builder/instrumentation"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/multisync/router"
)

var syncServerCmd = &cobra.Command{
	Use:     "sync-server",
	Short:   "Start the multi source sync server",
	Example: syncServerExample(),
	RunE: func(*cobra.Command, []string) (err error) {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		stopOtel, err := instrumentation.SetupOTelSDK(context.Background(), cfg, instrumentation.Sync)
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
		r, err := router.NewRouter(cfg)
		if err != nil {
			return fmt.Errorf("failed to init router: %w", err)
		}
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.Mirror.Port,
			},
			r,
		)
		server.Run()
		_ = stopOtel(context.Background())
		return nil
	},
}

func syncServerExample() string {
	return `
# for development
csghub-server sync sync-server
`
}
