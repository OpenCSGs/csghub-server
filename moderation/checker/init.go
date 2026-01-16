package checker

import (
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
)

var contentChecker sensitive.SensitiveChecker

func Init(config *config.Config) {
	if !config.SensitiveCheck.Enable {
		panic("SensitiveCheck is not enable")
	}
	//init aliyun green checker
	contentChecker = sensitive.NewChainChecker(config,
		sensitive.WithACAutomaton(sensitive.LoadFromConfig(config)),
		sensitive.WithMutableACAutomaton(sensitive.LoadFromDB()),
		sensitive.WithAliYunChecker())
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
}
