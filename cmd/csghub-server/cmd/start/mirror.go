package start

import (
	"fmt"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/router"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

var mirrorCmd = &cobra.Command{
	Use:     "mirror",
	Short:   "Start the reverse proxy server",
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
		r, err := router.NewMirrorRouter(cfg)
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

func mirrorExample() string {
	return `
# for development
csghub-server start rproxy
`
}
