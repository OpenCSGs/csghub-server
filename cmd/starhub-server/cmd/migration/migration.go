package migration

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/uptrace/bun/migrate"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/utils/console"
)

// verboseMode whether to show SQL detail
var verboseMode bool

// mockSession whether to insert mock user and session
var mockSession bool

func init() {
	Cmd.Flags().BoolVar(&verboseMode, "verbose", false, "whether to show SQL detail")
	migrateCmd.Flags().BoolVar(&mockSession, "dev-mock-session", false, "mock a user and its login session")
	Cmd.AddCommand(
		initCmd,
		migrateCmd,
		rollbackCmd,
		lockCmd,
		unlockCmd,
		createGoCmd,
		createSQLCmd,
		statusCmd,
		markAppliedCmd,
	)
}

var (
	migrator *migrate.Migrator
	db       *database.DB
)

var Cmd = &cobra.Command{
	Use:   "migration",
	Short: "run database migrations",
	Long:  "migration manage database schema, keeping it up-to-date with current application logic. Developer also uses migration to create new database migration during their development.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if verboseMode {
			err = os.Setenv("DB_DEBUG", "1")
			if err != nil {
				err = fmt.Errorf("setting ENV DB_DEBUG: %w", err)
				return
			}
		}

		config, err := config.LoadConfig()
		if err != nil {
			return
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		db, err = database.NewDB(cmd.Context(), dbConfig)
		if err != nil {
			err = fmt.Errorf("initializing DB connection: %w", err)
			return
		}
		migrator = database.NewMigrator(db)

		return
	},
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if db != nil {
			_ = db.Close()
		}
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "create migration tables",
	RunE: func(cmd *cobra.Command, args []string) error {
		return migrator.Init(cmd.Context())
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrate database",
	RunE: func(cmd *cobra.Command, args []string) error {
		group, err := migrator.Migrate(cmd.Context())
		if err != nil {
			return err
		}
		// if mockSession {
		// 	err = db.InsertMockUserAndSession(cmd.Context())
		// 	if err != nil {
		// 		err = fmt.Errorf("inserting mock user and session: %w", err)
		// 		return err
		// 	}
		// }
		if group.IsZero() {
			console.RenderSuccess("there are no new migrations to run (database is up to date)").Println()
			return nil
		}
		console.RenderSuccess(fmt.Sprintf("migrated to %s\n", group))
		return nil
	},
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "rollback the last migration group",
	RunE: func(cmd *cobra.Command, args []string) error {
		group, err := migrator.Rollback(cmd.Context())
		if err != nil {
			return err
		}
		if group.IsZero() {
			console.RenderSuccess("there are no groups to roll back").Println()
			return nil
		}
		console.RenderSuccess(fmt.Sprintf("rolled back %s\n", group))
		return nil
	},
}

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "lock migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		return migrator.Lock(cmd.Context())
	},
}

var unlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "unlock migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		return migrator.Unlock(cmd.Context())
	},
}

var createGoCmd = &cobra.Command{
	Use:   "create_go",
	Short: "create Go migration for developers",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.Join(args, "_")
		mf, err := migrator.CreateGoMigration(cmd.Context(), name)
		if err != nil {
			return err
		}
		fmt.Printf("created migration %s (%s)\n", mf.Name, mf.Path)
		return nil
	},
}

var createSQLCmd = &cobra.Command{
	Use:   "create_sql",
	Short: "create up and down SQL migrations for developers",
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.Join(args, "_")
		files, err := migrator.CreateSQLMigrations(cmd.Context(), name)
		if err != nil {
			return err
		}

		for _, mf := range files {
			fmt.Printf("created migration %s (%s)\n", mf.Name, mf.Path)
		}

		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "print migrations status",
	RunE: func(cmd *cobra.Command, args []string) error {
		ms, err := migrator.MigrationsWithStatus(cmd.Context())
		if err != nil {
			return err
		}
		fmt.Printf("migrations: %s\n", ms)
		fmt.Printf("unapplied migrations: %s\n", ms.Unapplied())
		fmt.Printf("last migration group: %s\n", ms.LastGroup())
		return nil
	},
}

var markAppliedCmd = &cobra.Command{
	Use:   "mark_applied",
	Short: "mark migrations as applied without actually running them",
	RunE: func(cmd *cobra.Command, args []string) error {
		group, err := migrator.Migrate(cmd.Context(), migrate.WithNopMigration())
		if err != nil {
			return err
		}
		if group.IsZero() {
			fmt.Printf("there are no new migrations to mark as applied\n")
			return nil
		}
		fmt.Printf("marked as applied %s\n", group)
		return nil
	},
}
