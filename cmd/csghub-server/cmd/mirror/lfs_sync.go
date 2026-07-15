package mirror

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	mirrorcomponent "opencsg.com/csghub-server/mirror/component"
	"opencsg.com/csghub-server/mirror/filter"
	"opencsg.com/csghub-server/mirror/lfssyncer"
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

		slog.Info("start temporal workflow")
		err = workflow.StartWorkflow(cfg, false)
		if err != nil {
			return err
		}
		defer temporal.Stop()

		lfsSyncer, err := lfssyncer.NewLfsSyncWorker(cfg)
		if err != nil {
			return err
		}

		lfsWorkClient, err := mirrorcomponent.NewLFSWorkClient(context.Background(), cfg.Database.DSN, mirrorcomponent.LFSWorkDeps{
			MirrorTaskStore: database.NewMirrorTaskJobStore(),
			Syncer:          lfsSyncer,
			RepoFilter:      filter.NewRepoFilter(cfg),
			MaxWorkers:      cfg.Mirror.WorkerNumber,
		})
		if err != nil {
			return fmt.Errorf("failed to create LFS workhub client: %w", err)
		}
		if err := lfsWorkClient.Start(context.Background()); err != nil {
			return fmt.Errorf("failed to start LFS workhub client: %w", err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := lfsWorkClient.Stop(ctx); err != nil {
				slog.Error("failed to stop LFS workhub client", slog.Any("error", err))
			}
		}()

		server.Run()
		return nil
	},
}

func lfsSyncExample() string {
	return `
# for development
csghub-server mirror lfs-sync
`
}
