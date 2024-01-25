package start

import (
	"fmt"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/router"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/docs"
)

var enableSwagger bool

func init() {
	serverCmd.Flags().BoolVar(&enableSwagger, "swagger", false, "Start swagger help docs")
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

		enableSwagger = enableSwagger || cfg.EnableSwagger

		if enableSwagger {
			//	@securityDefinitions.apikey ApiKey
			//	@in                         header
			//	@name                       Authorization
			//	@description                Bearer token
			docs.SwaggerInfo.Title = "CSGHub Server API"
			docs.SwaggerInfo.Description = "CSGHub Server API."
			docs.SwaggerInfo.Version = "1.0"
			docs.SwaggerInfo.Host = cfg.APIServer.ExternalHost
			docs.SwaggerInfo.BasePath = "/api/v1"
			docs.SwaggerInfo.Schemes = []string{"http", "https"}
		}

		// Check APIToken length
		if len(cfg.APIToken) < 128 {
			return fmt.Errorf("API token length is less than 128, please check")
		}
		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(cfg.Database.Driver),
			DSN:     cfg.Database.DSN,
		}
		database.InitDB(dbConfig)
		r, err := router.NewRouter(cfg, enableSwagger)
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
csghub-server start server
`
}
