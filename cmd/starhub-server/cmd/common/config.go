package common

import (
	"git-devops.opencsg.com/product/community/starhub-server/config"
	"github.com/kelseyhightower/envconfig"
)

func LoadConfig() (cfg *config.Config, err error) {
	cfg = new(config.Config)
	err = envconfig.Process("", cfg)
	if err != nil {
		return
	}

	return
}
