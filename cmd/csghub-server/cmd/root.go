package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/cmd/csghub-server/cmd/accounting"
	"opencsg.com/csghub-server/cmd/csghub-server/cmd/cron"
	"opencsg.com/csghub-server/cmd/csghub-server/cmd/deploy"
	"opencsg.com/csghub-server/cmd/csghub-server/cmd/logscan"
	"opencsg.com/csghub-server/cmd/csghub-server/cmd/migration"
	"opencsg.com/csghub-server/cmd/csghub-server/cmd/mirror"
	"opencsg.com/csghub-server/cmd/csghub-server/cmd/start"
	"opencsg.com/csghub-server/cmd/csghub-server/cmd/syncversion"
	"opencsg.com/csghub-server/cmd/csghub-server/cmd/trigger"
	"opencsg.com/csghub-server/cmd/csghub-server/cmd/user"
)

var (
	logLevel  string
	logFormat string
)

var RootCmd = &cobra.Command{
	Use:          "csghub-server",
	Short:        "Back-end API server for starhub.",
	SilenceUsage: true,
}

func init() {
	var err error
	defer func() {
		if err != nil {
			panic(err)
		}
	}()

	RootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "set log level to debug, info, warn, error or fatal (case-insensitive). default is INFO")
	RootCmd.PersistentFlags().StringVarP(&logFormat, "log-format", "f", "json", "set log format to json or text. default is json")
	RootCmd.DisableAutoGenTag = true

	cobra.OnInitialize(func() {
		setupLog(logLevel, logFormat)
	})

	RootCmd.AddCommand(
		migration.Cmd,
		start.Cmd,
		logscan.Cmd,
		trigger.Cmd,
		deploy.Cmd,
		cron.Cmd,
		mirror.Cmd,
		accounting.Cmd,
		syncversion.Cmd,
		user.Cmd,
	)
}

func setupLog(lvl, format string) {
	logLevel := slog.LevelInfo.Level()
	var logger *slog.Logger
	if len(lvl) > 0 {
		err := logLevel.UnmarshalText([]byte(lvl))
		// logLevel not change if unmarshall failed
		if err != nil {
			fmt.Println("input invalid log level, use default log level INFO")
		}
	}
	// TODO:log source file position
	opt := &slog.HandlerOptions{AddSource: false, Level: logLevel}
	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opt)
	default:
		handler = slog.NewTextHandler(os.Stdout, opt)
	}
	fmt.Printf("init logger, level: %s, format: %s\n", logLevel.String(), format)
	logger = slog.New(handler)
	slog.SetDefault(logger)
}
