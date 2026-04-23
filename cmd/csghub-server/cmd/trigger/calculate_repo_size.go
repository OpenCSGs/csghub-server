package trigger

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/log"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

var calculateRepoSizeCmd = &cobra.Command{
	Use:   "calculate-repo-size",
	Short: "calculate repository size by triggering workflow",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := config.LoadConfig()
		if err != nil {
			slog.Error("failed to load config", "err", err)
			return
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		if err := database.InitDB(dbConfig); err != nil {
			slog.Error("failed to initialize database", slog.Any("error", err))
			return
		}

		// Initialize temporal client
		workflowClient, err := temporal.NewClient(client.Options{
			HostPort: config.WorkFLow.Endpoint,
			Logger:   log.NewStructuredLogger(slog.Default()),
			ConnectionOptions: client.ConnectionOptions{
				GetSystemInfoTimeout: time.Duration(config.Temporal.GetSystemInfoTimeout) * time.Second,
			},
		}, "csghub-trigger")
		if err != nil {
			slog.Error("failed to create workflow client", "err", err)
			return
		}
		defer workflowClient.Close()

		ctx := context.Background()
		repoStore := database.NewRepoStore()
		offset := 0
		limit := 100    // Process in batches, get 100 at a time
		batchSize := 10 // Process 10 repositories per batch

		for {
			// Get repositories without associated RepositoryStatistics
			repos, err := repoStore.GetRepositoriesWithoutStatistics(ctx, limit, offset)
			if err != nil {
				slog.Error("failed to get repositories without statistics", "err", err)
				return
			}

			if len(repos) == 0 {
				slog.Info("no more repositories to process")
				break
			}

			slog.Info("processing repositories", "count", len(repos), "offset", offset)

			// Process in batches
			for i, repo := range repos {
				slog.Info("triggering calculate repo size workflow", "repo_id", repo.ID, "path", repo.Path)

				// Parse repo.Path into namespace/name
				parts := strings.Split(repo.Path, "/")
				if len(parts) != 2 {
					slog.Error("invalid repo path", "path", repo.Path)
					continue
				}
				namespace, name := parts[0], parts[1]

				// Build GiteaCallbackPushReq
				req := &types.GiteaCallbackPushReq{
					Repository: types.GiteaCallbackPushReq_Repository{
						FullName: fmt.Sprintf("%ss_%s/%s", strings.ToLower(string(repo.RepositoryType)), namespace, name),
						Private:  repo.Private,
					},
					Ref: "refs/heads/main", // Default to main branch
				}

				// Set up workflow options
				workflowOptions := client.StartWorkflowOptions{
					TaskQueue: workflow.HandlePushQueueName,
					ID:        fmt.Sprintf("calculate-repo-size-%d-%s", repo.ID, time.Now().Format("20060102-150405")),
				}

				// Execute workflow
				workflowRun, err := workflowClient.ExecuteWorkflow(ctx, workflowOptions, workflow.CalculateRepoSizeWorkflow, req)
				if err != nil {
					slog.Error("failed to trigger workflow", "err", err, "repo_id", repo.ID)
					continue
				}

				slog.Info("triggered workflow successfully", "repo_id", repo.ID, "workflow_id", workflowRun.GetID())

				// Take a break after each batch to avoid too frequent processing
				if (i+1)%batchSize == 0 {
					slog.Info("batch processed, taking a break")
					time.Sleep(5 * time.Second)
				}
			}

			offset += limit
			slog.Info("moving to next batch", "next_offset", offset)
		}
	},
}
