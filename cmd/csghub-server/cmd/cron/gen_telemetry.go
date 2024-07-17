package cron

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

var cmdGenTelemetry = &cobra.Command{
	Use:   "gen-telemetry",
	Short: "the cmd to generate telemetry data",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		config, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config,%w", err)
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		database.InitDB(dbConfig)
		if err != nil {
			return fmt.Errorf("initializing DB connection: %w", err)
		}
		ctx := context.WithValue(cmd.Context(), "config", config)
		cmd.SetContext(ctx)
		return
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		config, ok := ctx.Value("config").(*config.Config)
		if !ok {
			slog.Error("config not found in context")
			return
		}
		//Saas don't need to generate telemetry data
		if config.Saas {
			return
		}

		locker, err := cache.NewCache(ctx, cache.RedisConfig{
			Addr:     config.Redis.Endpoint,
			Username: config.Redis.User,
			Password: config.Redis.Password,
		})

		if err != nil {
			slog.Error("failed to initialize redis", "err", err)
			return
		}

		if err = locker.RunWhileLocked(ctx, "gen-telemetry-lock", 10*time.Minute, genTelemetry(config)); err != nil {
			slog.Error("failed to run gen telemetry", "err", err)
			return
		}
	},
}

func genTelemetry(config *config.Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		c, err := component.NewTelemetryComponent()
		if err != nil {
			return fmt.Errorf("failed to create mirror component, %w", err)
		}
		usage, err := c.GenUsageData(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate usage data, %w", err)
		}

		// save to local storage first
		err = c.SaveUsageData(ctx, usage)
		if err != nil {
			return fmt.Errorf("failed to save usage data to local storage, %w", err)
		}

		if !config.Telemetry.Enable {
			slog.Info("telemetry is not allowed to report")
			return nil
		}

		// send report to telemetry server
		hc := http.DefaultClient
		var data bytes.Buffer
		err = json.NewEncoder(&data).Encode(usage)
		if err != nil {
			return fmt.Errorf("failed to encode usage data, %w", err)
		}

		teleUrl, err := url.JoinPath(config.Telemetry.ReportURL, "usage")
		if err != nil {
			return fmt.Errorf("failed to join telemetry url, %w", err)
		}
		resp, err := hc.Post(teleUrl, "application/json", bytes.NewReader(data.Bytes()))
		if err != nil {
			return fmt.Errorf("failed to report usage data, %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			var respData bytes.Buffer
			io.Copy(&respData, resp.Body)
			return fmt.Errorf("telemetry api returns error,url:%s, status:%d, body:%s", teleUrl, resp.StatusCode, respData.String())
		}
		return nil
	}
}
