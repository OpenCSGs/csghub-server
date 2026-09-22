package sensitive

import (
	"log/slog"
	"strings"

	"opencsg.com/csghub-server/builder/providers/aliyun"
	"opencsg.com/csghub-server/builder/sensitive/internal"
	"opencsg.com/csghub-server/common/config"
	ss_type "opencsg.com/csghub-server/common/types/sensitive"
)

// ProviderName constants for consistency
const (
	ProviderACAutomaton        = "ac_automaton"
	ProviderMutableACAutomaton = "mutable_ac_automaton"
	ProviderAliyunGreen        = "aliyun_green"
	ProviderLLM                = "guard_llm"
)

type chainImpl struct {
	checkers []ss_type.SensitiveChecker
}

type ChainOption func(*config.Config, *chainImpl)

// WithAliYunPluginChecker adds a checker backed by the Aliyun checker plugin.
// The plugin process is started lazily because NewChainCheckerFromConfig is
// also used for short-lived activities.
func WithAliYunPluginChecker() ChainOption {
	return func(config *config.Config, c *chainImpl) {
		c.checkers = append(c.checkers, aliyun.NewPluginAliyunChecker(config))
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
func NewChainChecker(config *config.Config, opts ...ChainOption) ss_type.SensitiveChecker {
	c := &chainImpl{}
	for _, opt := range opts {
		opt(config, c)
	}
	return c
}

func NewChainCheckerWithCheckers(checkers ...ss_type.SensitiveChecker) ss_type.SensitiveChecker {
	return &chainImpl{
		checkers: checkers,
	}
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
func NewChainCheckerFromConfig(config *config.Config) ss_type.SensitiveChecker {
	var opts []ChainOption

	for _, provider := range config.SensitiveCheck.CheckChain {
		slog.Info("sensitive check provider", slog.String("provider", provider))
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
		return []ChainOption{WithAliYunPluginChecker()}
	default:
		if provider != "" {
			slog.Warn("unknown sensitive check provider ignored", slog.String("provider", provider))
		}
		return nil
	}
}
