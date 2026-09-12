package repository

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"github.com/riverqueue/river"
	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/rebac/factory"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

const repositoryDeletionWorkerStopTimeout = 10 * time.Second

var deletionWorkerCmd = &cobra.Command{
	Use:   "deletion-worker",
	Short: "Start the asynchronous repository deletion worker",
	RunE:  runDeletionWorker,
}

func runDeletionWorker(cmd *cobra.Command, _ []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := database.InitDB(database.DBConfig{
		Dialect: database.DatabaseDialect(cfg.Database.Driver),
		DSN:     cfg.Database.DSN,
	}); err != nil {
		return fmt.Errorf("initialize database: %w", err)
	}

	worker, err := buildRepositoryDeletionWorker(cfg)
	if err != nil {
		return err
	}
	workClient, err := workhub.NewWorkClient(cmd.Context(), cfg.Database.DSN, newRepositoryDeletionRiverConfig(worker, cfg.Mirror.WorkerNumber))
	if err != nil {
		return fmt.Errorf("create repository deletion work client: %w", err)
	}

	workerContext, stopSignals := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	if err := workClient.Start(workerContext); err != nil {
		return fmt.Errorf("start repository deletion work client: %w", err)
	}
	slog.Info("repository deletion worker started", slog.String("queue", workhub.RepositoryDeletionQueue))
	<-workerContext.Done()

	stopContext, cancel := context.WithTimeout(context.Background(), repositoryDeletionWorkerStopTimeout)
	defer cancel()
	if err := workClient.Stop(stopContext); err != nil {
		return fmt.Errorf("stop repository deletion work client: %w", err)
	}
	return nil
}

func buildRepositoryDeletionWorker(cfg *config.Config) (*component.RepositoryDeletionWorker, error) {
	gitServer, err := git.NewGitServer(cfg)
	if err != nil {
		return nil, fmt.Errorf("create Git server client: %w", err)
	}
	s3Client, err := s3.NewMinio(cfg)
	if err != nil {
		return nil, fmt.Errorf("create S3 client: %w", err)
	}
	authorizer, err := factory.NewAuthorizer()
	if err != nil {
		return nil, fmt.Errorf("create ReBAC authorizer: %w", err)
	}
	// Repository package removal only needs the package path snapshot and S3.
	// A repository store is intentionally unnecessary after the row is soft-deleted.
	packageCleaner := component.NewRepositoryPackageSyncer(cfg, nil, gitServer, s3Client)
	resourceCleaner := component.NewRepositoryDeletionResourceCleaner(
		cfg,
		database.NewLfsMetaObjectStore(),
		s3Client,
		packageCleaner,
	)
	mirrorCanceler := rpc.NewMirrorSvcClient(
		fmt.Sprintf("%s:%d", cfg.LfsSync.Host, cfg.LfsSync.Port),
		rpc.AuthWithApiKey(cfg.APIToken),
	)
	return component.NewRepositoryDeletionWorker(
		gitServer,
		resourceCleaner,
		authorizer,
		database.NewRepositoryAuthorizationStore(),
		database.NewRepositoryDeletionFinalizer(),
		database.NewRepositoryDeletionMirrorTaskStore(),
		mirrorCanceler,
	), nil
}

func newRepositoryDeletionRiverConfig(worker river.Worker[workhub.RepositoryDeletionArgs], maxWorkers int) *river.Config {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	return &river.Config{
		Queues: map[string]river.QueueConfig{
			workhub.RepositoryDeletionQueue: {MaxWorkers: maxWorkers},
		},
		Workers: workhub.NewWorkerRegistry(workhub.WorkerOverrides{
			RepositoryDeletion: worker,
		}),
	}
}
