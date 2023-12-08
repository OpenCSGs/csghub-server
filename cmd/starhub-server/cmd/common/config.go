package common

import (
	"github.com/kelseyhightower/envconfig"
	"opencsg.com/starhub-server/config"
)

func LoadConfig() (cfg *config.Config, err error) {
	cfg = new(config.Config)
	err = envconfig.Process("", cfg)
	if err != nil {
		return
	}

	return
}
