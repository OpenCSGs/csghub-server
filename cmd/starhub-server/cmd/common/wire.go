package common

import "git-devops.opencsg.com/product/community/starhub-server/config"

func ProvideConfig() (*config.Config, error) {
	return LoadConfig()
}
