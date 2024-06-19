package cron

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
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
		c, err := component.NewMultiSyncComponent(config)
		if err != nil {
			slog.Error("failed to create recom component", "err", err)
			return
		}
		mirrorTokenStore := database.NewMirrorTokenStore()
		mirrorToken, err := mirrorTokenStore.First(ctx)
		if err != nil {
			slog.Error("failed to find mirror token, error: %w", err)
			return
		}
		sc := multisync.FromOpenCSG(mirrorToken.Token)
		err = c.SyncAsClient(ctx, sc)
		if err != nil {
			slog.Error("failed to sync as client", "err", err)
		}
	},
}
