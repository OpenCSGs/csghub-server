package trigger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

var (
	count  int
	lastID int64
)

const lastIDKey = "migrate-repo-path-last-id"

func init() {
	migrateRepoPathCmd.Flags().IntVar(&count, "count", 0, "Number of repos to migrate repos to hashed paths. 0 means all. Default is 0.")
	migrateRepoPathCmd.Flags().Int64Var(&lastID, "last-id", 0, "Last ID to start from. 0 means start from the beginning.Default is 0.")
}

var migrateRepoPathCmd = &cobra.Command{
	Use:   "migrate-repo-path",
	Short: "Migrate repos to hashed paths",
	RunE: func(cmd *cobra.Command, args []string) error {
		lh := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelInfo,
		})
		l := slog.New(lh)
		slog.SetDefault(l)

		var err error
		var cfg *config.Config
		cfg, err = config.LoadConfig()
		if err != nil {
			return err
		}
		ctx := context.Background()
		cache, err := cache.NewCache(context.Background(), cache.RedisConfig{
			Addr:     cfg.Redis.Endpoint,
			Username: cfg.Redis.User,
			Password: cfg.Redis.Password,
		})
		if err != nil {
			return err
		}

		repoComponent, err := component.NewRepoComponent(cfg)
		if err != nil {
			return err
		}

		if lastID == 0 {
			last, err := cache.Get(ctx, lastIDKey)
			if err == nil {
				last, err := strconv.ParseInt(last, 10, 64)
				if err != nil {
					last = 0
				}
				lastID = last
			}
		}

		if count == 0 {
			lastID, err = repoComponent.BatchMigrateRepoToHashedPath(ctx, true, 100, lastID)
		} else {
			lastID, err = repoComponent.BatchMigrateRepoToHashedPath(ctx, false, count, lastID)
		}

		if err != nil {
			slog.Error("failed to migrate repo path", slog.Any("error", err))
			return err
		}

		err = cache.Set(ctx, lastIDKey, strconv.FormatInt(lastID, 10))
		if err != nil {
			return fmt.Errorf("failed to set cache: %w", err)
		}
		return nil
	},
}
