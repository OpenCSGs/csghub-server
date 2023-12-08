package common

import "opencsg.com/starhub-server/config"

func ProvideConfig() (*config.Config, error) {
	return LoadConfig()
}
