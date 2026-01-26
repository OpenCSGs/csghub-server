package component

import (
	"context"

	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type SensitiveComponent interface {
	PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*sensitive.CheckResult, error)
	PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*sensitive.CheckResult, error)
	PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*sensitive.CheckResult, error)
	PassStreamCheck(ctx context.Context, scenario types.SensitiveScenario, text, id string) (*sensitive.CheckResult, error)
	PassLLMQueryCheck(ctx context.Context, scenario types.SensitiveScenario, text, id string) (*sensitive.CheckResult, error)
}

type SensitiveComponentImpl struct {
	checker sensitive.SensitiveChecker
}

func NewSensitiveComponent(checker sensitive.SensitiveChecker) SensitiveComponent {
	return SensitiveComponentImpl{
		checker: checker,
	}
}

func NewSensitiveComponentFromConfig(config *config.Config) SensitiveComponent {
	return SensitiveComponentImpl{
		checker: sensitive.NewChainChecker(config,
			sensitive.WithACAutomaton(sensitive.LoadFromConfig(config)),
			sensitive.WithMutableACAutomaton(sensitive.LoadFromDB()),
			sensitive.WithAliYunChecker(),
		),
	}
}

func (c SensitiveComponentImpl) PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*sensitive.CheckResult, error) {
	return c.checker.PassTextCheck(ctx, scenario, text)
}

func (c SensitiveComponentImpl) PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*sensitive.CheckResult, error) {
	return c.checker.PassImageCheck(ctx, scenario, ossBucketName, ossObjectName)
}

func (c SensitiveComponentImpl) PassStreamCheck(ctx context.Context, scenario types.SensitiveScenario, text, id string) (*sensitive.CheckResult, error) {
	return c.checker.PassLLMCheck(ctx, scenario, text, id, "")
}

func (c SensitiveComponentImpl) PassLLMQueryCheck(ctx context.Context, scenario types.SensitiveScenario, text, id string) (*sensitive.CheckResult, error) {
	return c.checker.PassLLMCheck(ctx, scenario, text, "", id)
}

func (c SensitiveComponentImpl) PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*sensitive.CheckResult, error) {
	return c.checker.PassImageURLCheck(ctx, scenario, imageURL)
}
