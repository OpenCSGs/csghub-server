package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"opencsg.com/csghub-server/common/config"
	aliyunchecker "opencsg.com/csghub-server/plugins/aliyun_checker"
	"opencsg.com/csghub-server/plugins/aliyun_checker/internal/aliyun"
)

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   fmt.Sprintf("plugin_%s", aliyunchecker.PluginName),
		Output: os.Stderr,
		Level:  hclog.Debug,
	})

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("LoadConfig failed:", slog.Any("err", err))
		panic(err)
	}

	checker := newAliyunCheckerAdapter(aliyun.NewAliyunCheckerFromConfig(cfg))

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: aliyunchecker.HandshakeConfig,
		Plugins: map[string]plugin.Plugin{
			aliyunchecker.PluginName: aliyunchecker.NewPlugin(checker),
		},
		GRPCServer: plugin.DefaultGRPCServer,
		Logger:     logger,
	})

	os.Exit(0)
}
