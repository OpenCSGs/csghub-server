package start

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"opencsg.com/starhub-server/api/httpbase"
	"opencsg.com/starhub-server/api/router"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
)

var (
	serverEnableTunnel bool
	enableOpenBrowser  bool
	enableSwagger      bool
	enableUI           bool
)

func init() {
	serverCmd.Flags().BoolVar(&enableSwagger, "swagger", false, "Start swagger help docs")
	serverCmd.Flags().BoolVar(&enableUI, "ui", false, "enable frontend ui")
	serverCmd.Flags().BoolVar(&serverEnableTunnel, "tunnel", false, "automatic connection to UltraFox dev tunnel, and modifies the externalhost configuration")
	serverCmd.Flags().BoolVar(&enableOpenBrowser, "open-browser", false, "auto open swagger and ui in browser")
	Cmd.AddCommand(serverCmd)
}

var serverCmd = &cobra.Command{
	Use:     "server",
	Short:   "Start the API server",
	Example: serverExample(),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		//TODO:init logger by config
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug}))
		slog.SetDefault(logger)

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(cfg.Database.Driver),
			DSN:     cfg.Database.DSN,
		}
		database.InitDB(dbConfig)
		r, err := router.NewRouter(cfg)
		if err != nil {
			return err
		}
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.APIServer.Port,
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
starhub-server start server
`
}
