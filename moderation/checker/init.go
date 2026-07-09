package checker

import (
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

var contentChecker sensitive.SensitiveChecker

// imageCheckEnabled controls whether image files are sent to the remote
// moderation service. It is set from config during Init.
var imageCheckEnabled bool

// imageCheckScenario is the Aliyun Green scenario used for image moderation.
// It is set from config during Init, defaulting to ScenarioImageBaseLineCheck.
var imageCheckScenario types.SensitiveScenario

func Init(config *config.Config) {
	if !config.SensitiveCheck.Enable {
		panic("SensitiveCheck is not enable")
	}
	contentChecker = sensitive.NewChainCheckerFromConfig(config)
	imageCheckEnabled = config.SensitiveCheck.ImageCheckEnable
	imageCheckScenario = resolveImageCheckScenario(config)
}

// InitWithContentChecker supports custom sensitive checker, this func mostly used in unit test
func InitWithContentChecker(config *config.Config, checker sensitive.SensitiveChecker) {
	if !config.SensitiveCheck.Enable {
		panic("SensitiveCheck is not enable")
	}

	if checker == nil {
		panic("param checker can not be nil")
	}
	contentChecker = checker
	imageCheckEnabled = config.SensitiveCheck.ImageCheckEnable
	imageCheckScenario = resolveImageCheckScenario(config)
}

// resolveImageCheckScenario returns the configured image check scenario,
// falling back to ScenarioImageBaseLineCheck if not set.
func resolveImageCheckScenario(config *config.Config) types.SensitiveScenario {
	s := config.SensitiveCheck.ImageCheckScenario
	if s == "" {
		return types.ScenarioImageBaseLineCheck
	}
	return types.SensitiveScenario(s)
}
