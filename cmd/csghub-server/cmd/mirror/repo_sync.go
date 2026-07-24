package mirror

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/builder/instrumentation"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/config"
	mirrorcomponent "opencsg.com/csghub-server/mirror/component"
	"opencsg.com/csghub-server/mirror/reposyncer"
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
		stopOtel, err := instrumentation.SetupOTelSDK(context.Background(), cfg, instrumentation.Mirror)
		if err != nil {
			panic(err)
		}
		defer func() {
			if err := stopOtel(context.Background()); err != nil {
				slog.Error("failed to stop otel", slog.Any("error", err))
			}
		}()
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

		err = workflow.StartWorkflow(cfg, false)
		if err != nil {
			return err
		}
		defer temporal.Stop()

		repoSyncer, err := reposyncer.NewRepoSyncWorker(cfg)
		if err != nil {
			return err
		}

		jobClient, err := workhub.NewJobClient(context.Background(), database.GetDB().BunDB)
		if err != nil {
			return fmt.Errorf("failed to create workhub job client: %w", err)
		}

		repoWorkClient, err := mirrorcomponent.NewRepoWorkClient(context.Background(), cfg.Database.DSN, mirrorcomponent.RepoWorkDeps{
			MirrorTaskStore: database.NewMirrorTaskJobStore(),
			Syncer:          repoSyncer,
			LFSJobClient:    workhub.NewMirrorLFSJobClient(jobClient, workhub.MirrorJobClientConfig{MaxRetryCount: cfg.Mirror.MaxRetryCount}),
			MaxWorkers:      cfg.Mirror.WorkerNumber,
		})
		if err != nil {
			return fmt.Errorf("failed to create repo workhub client: %w", err)
		}
		if err := repoWorkClient.Start(context.Background()); err != nil {
			return fmt.Errorf("failed to start repo workhub client: %w", err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := repoWorkClient.Stop(ctx); err != nil {
				slog.Error("failed to stop repo workhub client", slog.Any("error", err))
			}
		}()

		server.Run()
		return nil
	},
}

func repoSyncExample() string {
	return `
# for development
csghub-server mirror repo-sync
`
}
