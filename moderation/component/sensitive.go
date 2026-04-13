package component

import (
	"context"
	"log/slog"
	"strings"

	gwtype "opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type CheckProvider string

const (
	CheckProviderACAutomaton        CheckProvider = "ac_automaton"
	CheckProviderMutableACAutomaton CheckProvider = "mutable_ac_automaton"
	CheckProviderAliyunGreen        CheckProvider = "aliyun_green"
	CheckProviderLLMOpenAI          CheckProvider = "guard_llm"
)

type SensitiveComponent interface {
	PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*sensitive.CheckResult, error)
	PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*sensitive.CheckResult, error)
	PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*sensitive.CheckResult, error)
	// PassStreamCheck check stream chunk text
	PassStreamCheck(ctx context.Context, req *types.LLMCheckRequest) (*sensitive.CheckResult, error)
	// PassLLMQueryCheck check LLM prompt text
	PassLLMQueryCheck(ctx context.Context, req *types.LLMCheckRequest) (*sensitive.CheckResult, error)
}

type SensitiveComponentImpl struct {
	checker sensitive.SensitiveChecker
	cfg     *config.Config
}

func NewSensitiveComponentFromConfig(config *config.Config) SensitiveComponent {
	var opts []sensitive.ChainOption

	for _, provider := range config.SensitiveCheck.CheckChain {
		p := strings.TrimSpace(provider)
		extendedOpts := sensitiveChainOption(config, p)
		if len(extendedOpts) > 0 {
			opts = append(opts, extendedOpts...)
			continue
		}
		switch p {
		case string(CheckProviderACAutomaton):
			opts = append(opts, sensitive.WithACAutomaton(sensitive.LoadFromConfig(config)))
		case string(CheckProviderMutableACAutomaton):
			opts = append(opts, sensitive.WithMutableACAutomaton(sensitive.LoadFromDB()))
		case string(CheckProviderAliyunGreen):
			opts = append(opts, sensitive.WithAliYunChecker())
		default:
			if p != "" {
				slog.Warn("unknown sensitive check provider ignored", slog.String("provider", p))
			}
		}
	}

	return SensitiveComponentImpl{
		checker: sensitive.NewChainChecker(config, opts...),
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
	req.ModelName = c.cfg.SensitiveCheck.LLM.GuardStreamModel
	req.Role = string(gwtype.RoleAssistant)
	return c.checker.PassLLMCheck(ctx, req)
}

func (c SensitiveComponentImpl) PassLLMQueryCheck(ctx context.Context, req *types.LLMCheckRequest) (*sensitive.CheckResult, error) {
	req.ModelName = c.cfg.SensitiveCheck.LLM.GuardModel
	if req.Stream {
		req.ModelName = c.cfg.SensitiveCheck.LLM.GuardStreamModel
	}
	req.Role = string(gwtype.RoleUser)
	return c.checker.PassLLMCheck(ctx, req)
}

func (c SensitiveComponentImpl) PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*sensitive.CheckResult, error) {
	return c.checker.PassImageURLCheck(ctx, scenario, imageURL)
}
