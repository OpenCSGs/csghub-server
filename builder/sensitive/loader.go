package sensitive

import (
	"opencsg.com/csghub-server/builder/sensitive/internal"
	"opencsg.com/csghub-server/common/config"
)

func LoadFromDB() internal.Loader {
	return internal.FromDatabase()
}

func LoadFromConfig(cfg *config.Config) internal.Loader {
	return internal.NewConfigLoader(cfg)
}
