package notification

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/notification/router"
	"opencsg.com/csghub-server/notification/workflow"
)

var launchCmd = &cobra.Command{
	Use:     "launch",
	Short:   "Launch notification server",
	Example: serverExample(),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		// Check APIToken length
		if len(cfg.APIToken) < 128 {
			return fmt.Errorf("API token length is less than 128, please check")
		}
		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(cfg.Database.Driver),
			DSN:     cfg.Database.DSN,
		}
		if err := database.InitDB(dbConfig); err != nil {
			slog.Error("failed to initialize database", slog.Any("error", err))
			return fmt.Errorf("database initialization failed: %w", err)
		}

		err = workflow.StartWorkflow(cfg)
		if err != nil {
			slog.Warn("failed to start workflow", slog.Any("error", err))
		}

		r, err := router.NewNotifierRouter(cfg)
		if err != nil {
			return fmt.Errorf("failed to init router: %w", err)
		}
		slog.Info("http server is running", slog.Any("port", cfg.Notification.Port))
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.Notification.Port,
			},
			r,
		)
		server.Run()

		return nil
	},
}

func serverExample() string {
	return `
# for development
csghub-server notification launch
`
}
