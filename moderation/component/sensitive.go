package component

import (
	"context"

	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/moderation/checker"
)

type SensitiveComponent interface {
	PassTextCheck(ctx context.Context, scenario sensitive.Scenario, text string) (*sensitive.CheckResult, error)
	PassImageCheck(ctx context.Context, scenario sensitive.Scenario, ossBucketName, ossObjectName string) (*sensitive.CheckResult, error)
	PassImageURLCheck(ctx context.Context, scenario sensitive.Scenario, imageURL string) (*sensitive.CheckResult, error)
	PassStreamCheck(ctx context.Context, scenario sensitive.Scenario, text, id string) (*sensitive.CheckResult, error)
	PassLLMQueryCheck(ctx context.Context, scenario sensitive.Scenario, text, id string) (*sensitive.CheckResult, error)
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
	checker := sensitive.NewAliyunGreenCheckerFromConfig(config)
	return SensitiveComponentImpl{
		checker: checker,
	}
}

func (c SensitiveComponentImpl) PassTextCheck(ctx context.Context, scenario sensitive.Scenario, text string) (*sensitive.CheckResult, error) {
	// do local check first
	localChecker := checker.GetLocalWordChecker()
	yes := localChecker.ContainsSensitiveWord(text)
	if yes {
		return &sensitive.CheckResult{
			IsSensitive: true,
		}, nil
	}

	return c.checker.PassTextCheck(ctx, scenario, text)
}

func (c SensitiveComponentImpl) PassImageCheck(ctx context.Context, scenario sensitive.Scenario, ossBucketName, ossObjectName string) (*sensitive.CheckResult, error) {
	return c.checker.PassImageCheck(ctx, scenario, ossBucketName, ossObjectName)
}

func (c SensitiveComponentImpl) PassStreamCheck(ctx context.Context, scenario sensitive.Scenario, text, id string) (*sensitive.CheckResult, error) {
	return c.checker.PassLLMCheck(ctx, scenario, text, id, "")
}

func (c SensitiveComponentImpl) PassLLMQueryCheck(ctx context.Context, scenario sensitive.Scenario, text, id string) (*sensitive.CheckResult, error) {
	return c.checker.PassLLMCheck(ctx, scenario, text, "", id)
}

func (c SensitiveComponentImpl) PassImageURLCheck(ctx context.Context, scenario sensitive.Scenario, imageURL string) (*sensitive.CheckResult, error) {
	return c.checker.PassImageURLCheck(ctx, scenario, imageURL)
}
