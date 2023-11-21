package start

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git-devops.opencsg.com/product/community/starhub-server/cmd/starhub-server/cmd/common"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/log"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "start",
	Short: "Start a service",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		//Ensure database schema is up-to-date
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()

		config, err := common.LoadConfig()
		if err != nil {
			return
		}

		dbConfig := model.DBConfig{
			Dialect: model.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		db, err := model.NewDB(cmd.Context(), dbConfig)
		if err != nil {
			err = fmt.Errorf("initializing DB connection: %w", err)
			return
		}
		migrator := model.NewMigrator(db)

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

func waitExitSignal(callback func()) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-sigs
	log.Info("received signal, start to shutdown...", log.Any("signal", sig))

	callback()
}
