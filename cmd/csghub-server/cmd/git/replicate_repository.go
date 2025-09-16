package git

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/git/gitserver/gitaly"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

var (
	replicateReq       = &gitaly.ProjectStorageCloneRequest{}
	lastRepoIDCacheKey = "replicate-repo-last-repo-id"
)

func init() {
	// Adding flags based on the struct fields
	replicateRepositoryCmd.Flags().StringVar(&replicateReq.CurrentGitalyAddress, "ca", "", "Current Gitaly address")
	replicateRepositoryCmd.Flags().StringVar(&replicateReq.CurrentGitalyToken, "ct", "", "Current Gitaly token")
	replicateRepositoryCmd.Flags().StringVar(&replicateReq.CurrentGitalyStorage, "cs", "", "Current Gitaly storage")
	replicateRepositoryCmd.Flags().StringVar(&replicateReq.NewGitalyAddress, "na", "", "New Gitaly address")
	replicateRepositoryCmd.Flags().StringVar(&replicateReq.NewGitalyToken, "nt", "", "New Gitaly token")
	replicateRepositoryCmd.Flags().StringVar(&replicateReq.NewGitalyStorage, "ns", "", "New Gitaly storage")
}

var replicateRepositoryCmd = &cobra.Command{
	Use:   "replicate-repository",
	Short: "transfer repository to the gitaly cluster",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		config, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config,%w", err)
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		if err := database.InitDB(dbConfig); err != nil {
			slog.Error("failed to initialize database", slog.Any("error", err))
			return fmt.Errorf("database initialization failed: %w", err)
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

		if config.GitServer.Type == types.GitServerTypeGitea {
			return
		}

		helper, err := gitaly.NewCloneStorageHelper(replicateReq)
		if err != nil {
			slog.Error("create helper failed", slog.Any("err", err))
			return
		}

		repoStore := database.NewRepoStore()
		cache, err := cache.NewCache(ctx, cache.RedisConfig{
			Addr:     config.Redis.Endpoint,
			Username: config.Redis.User,
			Password: config.Redis.Password,
		})
		if err != nil {
			slog.Error("create cache failed", slog.Any("err", err))
			return
		}

		batchSize := 1000
		for {
			lastIDString, err := cache.Get(ctx, lastRepoIDCacheKey)
			if err != nil {
				slog.Error("get cache failed", slog.Any("err", err))
			}
			lastID, err := strconv.ParseInt(lastIDString, 10, 64)
			if err != nil {
				lastID = 0
			}
			repos, err := repoStore.BatchGet(ctx, lastID, batchSize, &types.BatchGetFilter{})
			if err != nil {
				newError := fmt.Errorf("fail to get repos,error:%w", err)
				slog.Error(newError.Error())
				return
			}
			if len(repos) == 0 {
				break
			}
			for _, repo := range repos {
				slog.Info("replicate repository", slog.Any("repo", repo.Path), slog.Any("repo_type", repo.RepositoryType))
				err := helper.TransferRepoBundle(ctx, repo.GitalyPath(), hashedRepoRelativePath(repo), replicateReq)
				if err != nil {
					slog.Error("failed to replicate repository",
						slog.Any("error", err),
						slog.Any("repo", repo.Path),
						slog.Any("repo_type", repo.RepositoryType))
				} else {
					slog.Info("replicate repository successfully", slog.Any("repo", repo.Path), slog.Any("repo_type", repo.RepositoryType))
				}
				err = cache.Set(ctx, lastRepoIDCacheKey, strconv.Itoa(int(repo.ID)))
				if err != nil {
					slog.Error("set cache failed", slog.Any("err", err))
				}
			}
		}
	},
}

func hashedRepoRelativePath(repo database.Repository) string {
	return common.BuildHashedRelativePath(repo.ID)
}
