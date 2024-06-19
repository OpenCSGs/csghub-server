package start

import (
	"fmt"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/multisync/router"
)

var syncServerCmd = &cobra.Command{
	Use:     "sync-server",
	Short:   "Start the multi source sync server",
	Example: rproxyExample(),
	RunE: func(*cobra.Command, []string) (err error) {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(cfg.Database.Driver),
			DSN:     cfg.Database.DSN,
		}
		database.InitDB(dbConfig)
		r, err := router.NewRouter(cfg)
		if err != nil {
			return fmt.Errorf("failed to init router: %w", err)
		}
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.Mirror.Port,
			},
			r,
		)
		server.Run()

		return nil
	},
}

func syncServerExample() string {
	return `
# for development
csghub-server start sync-server
`
}
