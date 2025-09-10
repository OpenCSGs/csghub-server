package mirror

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mirror/manager"
	"opencsg.com/csghub-server/mirror/router"
)

var lfsSyncCmd = &cobra.Command{
	Use:     "lfs-sync",
	Short:   "Start the repoisotry lfs files sync server",
	Example: lfsSyncExample(),
	RunE: func(*cobra.Command, []string) (err error) {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
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
		slog.Info("http server is running", slog.Any("port", cfg.LfsSync.Port))
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.LfsSync.Port,
			},
			r,
		)
		go server.Run()

		slog.Info("start temporal workflow")
		err = workflow.StartWorkflow(cfg)
		if err != nil {
			return err
		}

		m, err := manager.GetManager(cfg)
		if err != nil {
			return fmt.Errorf("failed to get manager")
		}
		m.Start()

		return nil
	},
}

func lfsSyncExample() string {
	return `
# for development
csghub-server mirror lfs-sync
`
}
