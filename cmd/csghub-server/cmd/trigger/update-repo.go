package trigger

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type UpdateRepoCmdArg struct {
	RepoIDs []int64
}

var updateRepoCmdArg UpdateRepoCmdArg

func init() {
	updateRepoCmd.Flags().Int64SliceVar(&updateRepoCmdArg.RepoIDs, "repo-ids", []int64{}, "list of repo ids")
}

var updateRepoCmd = &cobra.Command{
	Use:   "update-repo",
	Short: "update repo",
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

		database.InitDB(dbConfig)
		if err != nil {
			slog.Error("initializing DB connection", "err", err)
			return
		}

		if !config.Saas {
			slog.Info("saas mode, skip sensitive check")
			return
		}

		ctx := context.Background()
		repoStore := database.NewRepoStore()
		for _, repoID := range updateRepoCmdArg.RepoIDs {
			repo, err := repoStore.FindById(ctx, repoID)
			if err != nil {
				slog.Error("failed to get repo", "err", err, "repo_id", repoID)
				continue
			}
			repo.Private = false
			repo.SensitiveCheckStatus = types.SensitiveCheckPass
			_, err = repoStore.UpdateRepo(ctx, *repo)
			if err != nil {
				slog.Error("failed to update repo", "err", err, "repo_id", repo.ID)
				continue
			}
			slog.Info("update repo success", "repo_id", repo.ID)
		}
	},
}
