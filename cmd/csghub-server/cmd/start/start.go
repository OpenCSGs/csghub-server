package start

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database/migrations"
	"opencsg.com/csghub-server/common/config"
)

func init() {
	Cmd.AddCommand(serverCmd)
	Cmd.AddCommand(rproxyCmd)
}

var Cmd = &cobra.Command{
	Use:   "start",
	Short: "Start a service",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		// Ensure database schema is up-to-date
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()

		config, err := config.LoadConfig()
		if err != nil {
			return
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		db, err := database.NewDB(ctx, dbConfig)
		if err != nil {
			err = fmt.Errorf("initializing DB connection: %w", err)
			return
		}
		migrator := migrations.NewMigrator(db)

		// migration init
		err = migrator.Init(ctx)
		if err != nil {
			slog.Error("failed to init migration", slog.Any("error", err))
			return
		}

		// migration migrate
		group, err := migrator.Migrate(ctx)
		if err != nil {
			return
		}
		if group.IsZero() {
			slog.Info("there are no new migrations to run (database is up to date)")
			return
		}
		slog.Info(fmt.Sprintf("migrated to %s", group))

		return
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}
