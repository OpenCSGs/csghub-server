package mirror

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
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

		c, err := component.NewMirrorComponent(config)
		if err != nil {
			slog.Error("failed to create mirror component", "err", err)
			return
		}
		err = c.CheckMirrorProgress(ctx)
		if err != nil {
			slog.Error("failed to check mirror progess", "err", err)
			return
		}
	},
}
