package sensitive

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/builder/sensitive/internal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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

func (c *chainImpl) PassLLMCheck(ctx context.Context, scenario types.SensitiveScenario, text string, sessionId string, accountId string) (*CheckResult, error) {
	for _, checker := range c.checkers {
		res, err := checker.PassLLMCheck(ctx, scenario, text, sessionId, accountId)
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
