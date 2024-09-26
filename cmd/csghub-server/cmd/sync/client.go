package sync

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

const (
	resourceName   = "sync-as-client"
	expirationTime = 1 * time.Hour
)

var cmdSyncAsClient = &cobra.Command{
	Use:   "sync-as-client",
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
			c, err := component.NewMultiSyncComponent(config)
			if err != nil {
				slog.Error("failed to create multi sync component", "err", err)
				return err
			}
			syncClientSettingStore := database.NewSyncClientSettingStore()
			setting, err := syncClientSettingStore.First(ctx)
			if err != nil {
				slog.Error("failed to find sync client setting, error: %w", err)
				return err
			}
			apiDomain := config.MultiSync.SaasAPIDomain
			sc := multisync.FromOpenCSG(apiDomain, setting.Token)
			return c.SyncAsClient(ctx, sc)
		})
		if err != nil {
			slog.Error("failed to sync as client", "err", err)
		}
	},
}
