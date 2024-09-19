package start

import (
	"context"
	"fmt"
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

		db, err := database.NewDB(cmd.Context(), dbConfig)
		if err != nil {
			err = fmt.Errorf("initializing DB connection: %w", err)
			return
		}
		migrator := migrations.NewMigrator(db)

		status, err := migrator.MigrationsWithStatus(ctx)
		if err != nil {
			err = fmt.Errorf("listing database migration status: %w", err)
			return
		}

		unapplied := status.Unapplied()
		if len(unapplied) > 0 {
			err = fmt.Errorf("abort to start, there are unapplied database migrations: %s", unapplied)
			return
		}

		return
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}
