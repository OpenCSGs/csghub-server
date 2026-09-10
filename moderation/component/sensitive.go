package component

import (
	"context"
	"fmt"

	gwtype "opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type SensitiveComponent interface {
	PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*sensitive.CheckResult, error)
	PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*sensitive.CheckResult, error)
	PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*sensitive.CheckResult, error)
	// PassStreamCheck check stream chunk text
	PassStreamCheck(ctx context.Context, req *types.LLMCheckRequest) (*sensitive.CheckResult, error)
	// PassLLMQueryCheck check LLM prompt text
	PassLLMQueryCheck(ctx context.Context, req *types.LLMCheckRequest) (*sensitive.CheckResult, error)
	// SubmitMediaModeration submits one audio/video URL for asynchronous
	// moderation and returns the provider task handle.
	SubmitMediaModeration(ctx context.Context, req types.MediaModerationRequest) (*types.MediaModerationSubmission, error)
	// QueryMediaModerationResult polls a submitted media moderation task.
	QueryMediaModerationResult(ctx context.Context, req types.MediaModerationRequest) (*types.MediaModerationResult, error)
}

type SensitiveComponentImpl struct {
	checker sensitive.SensitiveChecker
	cfg     *config.Config
}

func NewSensitiveComponentFromConfig(config *config.Config) SensitiveComponent {
	return SensitiveComponentImpl{
		checker: sensitive.NewChainCheckerFromConfig(config),
		cfg:     config,
	}
}

func (c SensitiveComponentImpl) PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*sensitive.CheckResult, error) {
	return c.checker.PassTextCheck(ctx, scenario, text)
}

func (c SensitiveComponentImpl) PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*sensitive.CheckResult, error) {
	return c.checker.PassImageCheck(ctx, scenario, ossBucketName, ossObjectName)
}

func (c SensitiveComponentImpl) PassStreamCheck(ctx context.Context, req *types.LLMCheckRequest) (*sensitive.CheckResult, error) {
	req.Stream = true
	req.IsAppendSystemPromot = false
	req.Role = string(gwtype.RoleAssistant)
	return c.checker.PassLLMCheck(ctx, req)
}

func (c SensitiveComponentImpl) PassLLMQueryCheck(ctx context.Context, req *types.LLMCheckRequest) (*sensitive.CheckResult, error) {
	req.IsAppendSystemPromot = false
	if req.Stream {
		req.IsAppendSystemPromot = true
	}
	req.Role = string(gwtype.RoleUser)
	return c.checker.PassLLMCheck(ctx, req)
}

func (c SensitiveComponentImpl) PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*sensitive.CheckResult, error) {
	return c.checker.PassImageURLCheck(ctx, scenario, imageURL)
}

func (c SensitiveComponentImpl) SubmitMediaModeration(ctx context.Context, req types.MediaModerationRequest) (*types.MediaModerationSubmission, error) {
	mc, ok := c.checker.(sensitive.MediaSensitiveChecker)
	if !ok {
		return nil, fmt.Errorf("configured sensitive checker does not support media moderation")
	}
	return mc.SubmitMediaModeration(ctx, req)
}

func (c SensitiveComponentImpl) QueryMediaModerationResult(ctx context.Context, req types.MediaModerationRequest) (*types.MediaModerationResult, error) {
	mc, ok := c.checker.(sensitive.MediaSensitiveChecker)
	if !ok {
		return nil, fmt.Errorf("configured sensitive checker does not support media moderation")
	}
	return mc.QueryMediaModerationResult(ctx, req)
}
