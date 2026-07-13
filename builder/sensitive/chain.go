package sensitive

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"opencsg.com/csghub-server/builder/sensitive/internal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// ProviderName constants for consistency
const (
	ProviderACAutomaton        = "ac_automaton"
	ProviderMutableACAutomaton = "mutable_ac_automaton"
	ProviderAliyunGreen        = "aliyun_green"
	ProviderLLM                = "guard_llm"
)

type chainImpl struct {
	checkers []SensitiveChecker
}

type ChainOption func(*config.Config, *chainImpl)

// WithAliYunChecker adds an AliYun sensitive checker to the chain
// if the configuration is provided
func WithAliYunChecker() ChainOption {
	return func(config *config.Config, c *chainImpl) {
		if config.SensitiveCheck.AccessKeyID != "" &&
			config.SensitiveCheck.AccessKeySecret != "" &&
			config.SensitiveCheck.Region != "" {
			checker := NewAliyunGreenCheckerFromConfig(config)
			checker.initS3Client(config)
			c.checkers = append(c.checkers, checker)
		} else {
			slog.Warn("sensitive config for AliYun modereation service not set")
		}
	}
}

// WithACAutomaton adds an Aho-Corasick automaton sensitive checker to the chain
func WithACAutomaton(loader internal.Loader) ChainOption {
	return func(config *config.Config, c *chainImpl) {
		data, err := loader.Load()
		if err != nil {
			slog.Error("Failed to load sensitive data",
				slog.String("error", err.Error()))
		}
		checker := NewACAutomation(data)
		c.checkers = append(c.checkers, checker)
	}
}

// WithMutableACAutomaton is now using ImmutableACAutomation
// For backward compatibility, we keep the function name but use ImmutableAC
func WithMutableACAutomaton(loader internal.Loader) ChainOption {
	return func(config *config.Config, c *chainImpl) {
		mutableACNode := NewMutableACAutomation(loader)
		c.checkers = append(c.checkers, mutableACNode)
	}
}

// NewChainChecker create a chain sensitive checker
//
// It will run all checkers in order by the options provided
func NewChainChecker(config *config.Config, opts ...ChainOption) SensitiveChecker {
	c := &chainImpl{}
	for _, opt := range opts {
		opt(config, c)
	}
	return c
}

func NewChainCheckerWithCheckers(checkers ...SensitiveChecker) SensitiveChecker {
	return &chainImpl{
		checkers: checkers,
	}
}

func (c *chainImpl) PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*CheckResult, error) {
	for _, checker := range c.checkers {
		res, err := checker.PassTextCheck(ctx, scenario, text)
		if err != nil {
			return nil, err
		}
		if res.IsSensitive {
			// If any checker detects sensitivity, return immediately
			return res, nil
		}
	}
	return &CheckResult{IsSensitive: false}, nil
}

func (c *chainImpl) PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*CheckResult, error) {
	for _, checker := range c.checkers {
		res, err := checker.PassImageCheck(ctx, scenario, ossBucketName, ossObjectName)
		if err != nil {
			return nil, err
		}
		if res.IsSensitive {
			// If any checker detects sensitivity, return immediately
			return res, nil
		}
	}
	return &CheckResult{IsSensitive: false}, nil
}

func (c *chainImpl) PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*CheckResult, error) {
	for _, checker := range c.checkers {
		res, err := checker.PassImageURLCheck(ctx, scenario, imageURL)
		if err != nil {
			return nil, err
		}
		if res.IsSensitive {
			// If any checker detects sensitivity, return immediately
			return res, nil
		}
	}
	return &CheckResult{IsSensitive: false}, nil
}

func (c *chainImpl) PassImageStreamCheck(ctx context.Context, scenario types.SensitiveScenario, reader io.Reader) (*CheckResult, error) {
	for _, checker := range c.checkers {
		res, err := checker.PassImageStreamCheck(ctx, scenario, reader)
		if err != nil {
			return nil, err
		}
		if res.IsSensitive {
			// If any checker detects sensitivity, return immediately
			return res, nil
		}
	}
	return &CheckResult{IsSensitive: false}, nil
}

func (c *chainImpl) PassLLMCheck(ctx context.Context, req *types.LLMCheckRequest) (*CheckResult, error) {
	for _, checker := range c.checkers {
		res, err := checker.PassLLMCheck(ctx, req)
		if err != nil {
			return nil, err
		}
		if res.IsSensitive {
			// If any checker detects sensitivity, return immediately
			return res, nil
		}
	}
	return &CheckResult{IsSensitive: false}, nil
}

// AdvanceOptionsFunc allows external packages (e.g., EE versions) to register
// additional ChainOptions for specific providers, overriding the built-in defaults.
// When the returned slice is non-empty, the built-in default provider switch is skipped.
type AdvanceOptionsFunc func(config *config.Config, provider string) []ChainOption

var advanceOptionsFunc AdvanceOptionsFunc

// RegisterAdvanceOptions allows EE/SaaS versions to register advanced provider options
// that take precedence over the built-in defaults.
func RegisterAdvanceOptions(fn AdvanceOptionsFunc) {
	advanceOptionsFunc = fn
}

// NewChainCheckerFromConfig creates a chain sensitive checker from config's CheckChain.
//
// For each provider in the check chain, it first consults the registered AdvanceOptionsFunc.
// If the advanced function returns non-nil options, those are used and the built-in
// defaults are skipped (continue). Otherwise, the built-in default provider switch is applied.
func NewChainCheckerFromConfig(config *config.Config) SensitiveChecker {
	var opts []ChainOption

	for _, provider := range config.SensitiveCheck.CheckChain {
		p := strings.TrimSpace(provider)
		if advanceOpts := loadAdvanceCheckOpts(config, p); advanceOpts != nil {
			opts = append(opts, advanceOpts...)
			continue
		}
		opts = append(opts, defaultCheckOpts(config, p)...)
	}

	return NewChainChecker(config, opts...)
}

// loadAdvanceCheckOpts attempts to resolve provider-specific options via the registered
// AdvanceOptionsFunc. Returns nil if no advanced options are registered or the
// registered function returns no options for this provider.
func loadAdvanceCheckOpts(config *config.Config, provider string) []ChainOption {
	if advanceOptionsFunc == nil {
		return nil
	}
	return advanceOptionsFunc(config, provider)
}

// defaultCheckOpts resolves a provider to its built-in ChainOption.
// If the provider is unrecognized, a warning is logged and nil is returned.
func defaultCheckOpts(config *config.Config, provider string) []ChainOption {
	switch provider {
	case ProviderACAutomaton:
		return []ChainOption{WithACAutomaton(LoadFromConfig(config))}
	case ProviderMutableACAutomaton:
		return []ChainOption{WithMutableACAutomaton(LoadFromDB())}
	case ProviderAliyunGreen:
		return []ChainOption{WithAliYunChecker()}
	default:
		if provider != "" {
			slog.Warn("unknown sensitive check provider ignored", slog.String("provider", provider))
		}
		return nil
	}
}
