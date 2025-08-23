package mirror

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mirror"
	"opencsg.com/csghub-server/mirror/router"
)

var repoSyncCmd = &cobra.Command{
	Use:     "repo-sync",
	Short:   "Start the repoisotry sync server",
	Example: repoSyncExample(),
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
		slog.Info("http server is running", slog.Any("port", cfg.RepoSync.Port))
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.RepoSync.Port,
			},
			r,
		)
		go server.Run()

		// Exception recovery for mirrors.
		mirrorStore := database.NewMirrorStore()
		err = mirrorStore.Recover(context.Background())
		if err != nil {
			return fmt.Errorf("failed to recover mirrors: %w", err)
		}

		err = workflow.StartWorkflow(cfg)
		if err != nil {
			return err
		}

		repoSyncer, err := mirror.NewRepoSyncWorker(cfg, cfg.Mirror.WorkerNumber)
		if err != nil {
			return err
		}

		repoSyncer.Run()

		temporal.Stop()

		return nil
	},
}

func repoSyncExample() string {
	return `
# for development
csghub-server mirror repo-sync
`
}
