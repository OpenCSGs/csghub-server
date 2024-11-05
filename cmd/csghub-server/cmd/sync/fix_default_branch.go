package sync

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

var cmdFixDefaultBranch = &cobra.Command{
	Use:   "fix-default-branch",
	Short: "the cmd to sync repos like models and datasets from remote server like OpenCSG",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		config, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config,%w", err)
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		database.InitDB(dbConfig)
		if err != nil {
			return fmt.Errorf("initializing DB connection: %w", err)
		}
		ctx := context.WithValue(cmd.Context(), "config", config)
		cmd.SetContext(ctx)
		return
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		config, ok := ctx.Value("config").(*config.Config)
		if !ok {
			slog.Error("config not found in context")
			return
		}

		if config.Saas {
			return
		}

		if !config.MultiSync.Enabled {
			return
		}

		repoStore := database.NewRepoStore()
		repositories, err := repoStore.FindByRepoSource(ctx, types.OpenCSGSource)
		if err != nil {
			slog.Error("failed to find repositories from OpenCSG, error: %w", err)
			return
		}
		if len(repositories) == 0 {
			slog.Info("no repositories found from OpenCSG")
			return
		}
		syncClientSettingStore := database.NewSyncClientSettingStore()
		setting, err := syncClientSettingStore.First(ctx)
		if err != nil {
			slog.Error("failed to find sync client setting, error: %w", err)
			return
		}
		apiDomain := config.MultiSync.SaasAPIDomain
		sc := multisync.FromOpenCSG(apiDomain, setting.Token)
		for _, repository := range repositories {
			var defaultBranch string
			repoPath := strings.TrimPrefix(repository.Path, types.OpenCSGPrefix)
			if repository.RepositoryType == types.ModelRepo {
				modelInfo, err := sc.ModelInfo(ctx, types.SyncVersion{RepoPath: repoPath})
				if err != nil {
					slog.Error("failed to get model info from OpenCSG Saas", slog.String("repo_path", repoPath), slog.Any("error", err))
					continue
				}
				defaultBranch = modelInfo.DefaultBranch
			} else if repository.RepositoryType == types.DatasetRepo {
				datasetInfo, err := sc.DatasetInfo(ctx, types.SyncVersion{RepoPath: repoPath})
				if err != nil {
					slog.Error("failed to get dataset info from OpenCSG Saas", slog.String("repo_path", repoPath), slog.Any("error", err))
					continue
				}
				defaultBranch = datasetInfo.DefaultBranch
			}
			repository.DefaultBranch = defaultBranch
			_, err = repoStore.UpdateRepo(ctx, repository)
			if err != nil {
				slog.Error("failed to update repository", slog.String("repo_path", repoPath), slog.Any("error", err))
				continue
			}
		}
	},
}
