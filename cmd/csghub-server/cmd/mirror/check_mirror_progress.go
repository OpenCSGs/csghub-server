package mirror

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

const (
	resourceName   = "check-mirror-progress"
	expirationTime = 1 * time.Hour
)

var resync bool

func init() {
	checkMirrorProgress.Flags().BoolVar(&resync, "resync", false, "the path of the file")
}

var checkMirrorProgress = &cobra.Command{
	Use:   "check-mirror-progress",
	Short: "the cmd to check mirror progress",
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
		locker, err := cache.NewCache(ctx, cache.RedisConfig{
			Addr:     config.Redis.Endpoint,
			Username: config.Redis.User,
			Password: config.Redis.Password,
		})

		if err != nil {
			slog.Error("failed to initialize redis", "err", err)
			return
		}

		err = locker.RunWhileLocked(ctx, resourceName, expirationTime, func(ctx context.Context) error {
			c, err := component.NewMirrorComponent(config)
			if err != nil {
				slog.Error("failed to create mirror component", "err", err)
				return err
			}

			err = c.CheckMirrorProgress(ctx)
			if err != nil {
				slog.Error("failed to check mirror progress", "err", err)
				return err
			}
			return nil
		})
		if err != nil {
			slog.Error("failed to check mirror progress", "err", err)
			return
		}

	},
}
