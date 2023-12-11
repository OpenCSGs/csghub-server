package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"opencsg.com/starhub-server/cmd/starhub-server/cmd/migration"
	"opencsg.com/starhub-server/cmd/starhub-server/cmd/start"
	"opencsg.com/starhub-server/common/log"
	"opencsg.com/starhub-server/version"
)

var (
	cfgFile   string
	logLevel  string
	logFormat string
)

var RootCmd = &cobra.Command{
	Use:          "starhub-server",
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

	RootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "log level (debug, info, warn, error, fatal)")
	RootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "console", "json, console")
	RootCmd.DisableAutoGenTag = true

	cobra.OnInitialize(func() {
		logFields := log.WithFields(log.String("git-revision", version.GitRevision),
			log.String("version", version.StarhubAPIVersion))

		var lv log.Level
		lv, err = log.ParseLevel(logLevel)
		if err != nil {
			err = fmt.Errorf("parsing log level: %w", err)
			return
		}
		log.Init("starhub-server", lv, logFields, log.WithEncoding(logFormat))
	})

	RootCmd.AddCommand(
		migration.Cmd,
		start.Cmd,
	)
}
