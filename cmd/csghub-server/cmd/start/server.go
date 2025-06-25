package start

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/api/router"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database/migrations"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/docs"
)

var enableSwagger bool

func init() {
	serverCmd.Flags().BoolVar(&enableSwagger, "swagger", false, "Start swagger help docs")
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
			publicDomain, err := url.Parse(cfg.APIServer.PublicDomain)
			if err != nil {
				return fmt.Errorf("failed to parse api server public domain: %v", err)
			}
			docs.SwaggerInfo.Title = "CSGHub Server API"
			docs.SwaggerInfo.Description = "CSGHub Server API."
			docs.SwaggerInfo.Version = "1.0"
			docs.SwaggerInfo.Host = publicDomain.Host
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

		migrator := migrations.NewMigrator(database.GetDB())

		slog.Info("run migration")
		ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
		defer cancel()
		// migration init
		err = migrator.Init(ctx)
		if err != nil {
			slog.Error("failed to init migration", slog.Any("error", err))
			return fmt.Errorf("failed to init migration: %w", err)
		}

		// migration migrate
		group, err := migrator.Migrate(ctx)
		if err != nil {
			return fmt.Errorf("failed to migrate: %w", err)
		}
		if group.IsZero() {
			slog.Info("there are no new migrations to run (database is up to date)")
		}
		slog.Info(fmt.Sprintf("migrated to %s", group))

		err = event.InitEventPublisher(cfg, nil)
		if err != nil {
			return fmt.Errorf("fail to initialize message queue, %w", err)
		}
		s3Internal := len(cfg.S3.InternalEndpoint) > 0
		err = deploy.Init(common.DeployConfig{
			ImageBuilderURL:         cfg.Space.BuilderEndpoint,
			ImageRunnerURL:          cfg.Space.RunnerEndpoint,
			MonitorInterval:         10 * time.Second,
			InternalRootDomain:      cfg.Space.InternalRootDomain,
			SpaceDeployTimeoutInMin: cfg.Space.DeployTimeoutInMin,
			ModelDeployTimeoutInMin: cfg.Model.DeployTimeoutInMin,
			ModelDownloadEndpoint:   cfg.Model.DownloadEndpoint,
			PublicRootDomain:        cfg.Space.PublicRootDomain,
			S3Internal:              s3Internal,
			IsMasterHost:            cfg.IsMasterHost,
		})
		if err != nil {
			return fmt.Errorf("failed to init deploy: %w", err)
		}

		err = workflow.StartWorkflow(cfg)
		if err != nil {
			return err
		}
		router.RunServer(cfg, enableSwagger)

		return nil
	},
}

func serverExample() string {
	return `
# for development
csghub-server start server
`
}
