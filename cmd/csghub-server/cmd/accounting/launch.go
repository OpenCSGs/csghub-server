package accounting

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/instrumentation"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/accounting/consumer"
	"opencsg.com/csghub-server/accounting/router"
	"opencsg.com/csghub-server/api/httpbase"
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/i18n"
	"opencsg.com/csghub-server/mq"
)

var launchCmd = &cobra.Command{
	Use:     "launch",
	Short:   "Launch accounting server",
	Example: serverExample(),
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		cfg, err := config.LoadConfig()
		if err != nil {
			return err
		}
		slog.Debug("config", slog.Any("data", cfg))
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

		mqHandler, err := mq.GetOrInit(cfg)
		if err != nil {
			return fmt.Errorf("fail to build message queue handler: %w", err)
		}

		mqFactory, err := bldmq.GetOrInitMessageQueueFactory(cfg)
		if err != nil {
			return fmt.Errorf("failed to creating message queue factory: %w", err)
		}

		// Do metering
		meter := consumer.NewMetering(mqHandler, cfg)
		meter.Run()

		err = createAdvancedConsumer(cfg, mqHandler, mqFactory)
		if err != nil {
			return fmt.Errorf("failed to create advanced consumer: %w", err)
		}

		i18n.InitLocalizersFromEmbedFile()

		stopOtel, err := instrumentation.SetupOTelSDK(context.Background(), cfg, instrumentation.Account)
		if err != nil {
			panic(err)
		}

		r, err := router.NewAccountRouter(cfg, mqHandler, mqFactory)
		if err != nil {
			return fmt.Errorf("failed to init router: %w", err)
		}
		slog.Info("http server is running", slog.Any("port", cfg.Accounting.Port))
		server := httpbase.NewGracefulServer(
			httpbase.GraceServerOpt{
				Port: cfg.Accounting.Port,
			},
			r,
		)
		server.Run()
		_ = stopOtel(context.Background())
		return nil
	},
}

func serverExample() string {
	return `
# for development
csghub-server accounting launch
`
}
