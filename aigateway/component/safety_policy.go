package component

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

type SensitivePolicy interface {
	CheckChatSensitive(ctx context.Context, model *types.Model, messages []openai.ChatCompletionMessageParamUnion, userUUID string, stream bool, provider string) (bool, *rpc.CheckResult, error)
	CheckResponsesSensitive(ctx context.Context, model *types.Model, promptText string, userUUID string, stream bool, provider string) (bool, *rpc.CheckResult, error)
}

type sensitivePolicyImpl struct {
	moderation    Moderation
	whitelistRule database.RepositoryFileCheckRuleStore
}

func NewSensitivePolicy(moderation Moderation, whitelistRule database.RepositoryFileCheckRuleStore) SensitivePolicy {
	return &sensitivePolicyImpl{
		moderation:    moderation,
		whitelistRule: whitelistRule,
	}
}

func (s *sensitivePolicyImpl) CheckChatSensitive(ctx context.Context, model *types.Model, messages []openai.ChatCompletionMessageParamUnion, userUUID string, stream bool, provider string) (bool, *rpc.CheckResult, error) {
	enabled, key, err := s.prepareSensitiveCheck(ctx, model, userUUID, provider)
	if err != nil || !enabled {
		return false, nil, err
	}

	result, err := s.moderation.CheckChatPrompts(ctx, messages, key, stream)
	if err != nil {
		return false, nil, fmt.Errorf("failed to call moderation error:%w", err)
	}
	return true, result, nil
}

// CheckResponsesSensitive mirrors CheckChatSensitive but takes a pre-assembled
// prompt string instead of an OpenAI SDK message slice. The Responses API path
// flattens Instructions + Input into a single string before the check, so the
// same gate logic applies (NeedSensitiveCheck, namespace whitelist) without
// requiring the caller to map back into the SDK shape.
func (s *sensitivePolicyImpl) CheckResponsesSensitive(ctx context.Context, model *types.Model, promptText string, userUUID string, stream bool, provider string) (bool, *rpc.CheckResult, error) {
	enabled, key, err := s.prepareSensitiveCheck(ctx, model, userUUID, provider)
	if err != nil || !enabled {
		return false, nil, err
	}

	if strings.TrimSpace(promptText) == "" {
		return true, &rpc.CheckResult{IsSensitive: false}, nil
	}

	mode := types.TextModerationModeNonStream
	if stream {
		mode = types.TextModerationModeStream
	}
	result, err := s.moderation.CheckText(ctx, types.TextModerationRequest{
		Content: promptText,
		Key:     key,
		Phase:   types.TextModerationPhasePrompt,
		Mode:    mode,
	})
	if err != nil {
		return false, nil, fmt.Errorf("failed to call moderation error:%w", err)
	}
	return true, result, nil
}

func (s *sensitivePolicyImpl) prepareSensitiveCheck(ctx context.Context, model *types.Model, userUUID string, provider string) (bool, string, error) {
	if model == nil || !model.NeedSensitiveCheck || s.moderation == nil {
		return false, "", nil
	}
	if s.whitelistRule != nil {
		namespaceTargets := BuildNamespaceTargets(model.ID, provider)
		rules, err := s.whitelistRule.ListBySensitiveCheckTargets(ctx, namespaceTargets, model.ID)
		if err != nil {
			return false, "", fmt.Errorf("failed to query white list rules: %w", err)
		}
		if len(rules) != 0 {
			slog.DebugContext(ctx, "Skip Sensitive check with white list", slog.Any("rule", rules[0]))
			return false, "", nil
		}
	}

	return true, fmt.Sprintf("%s:%s", userUUID, model.ID), nil
}

func BuildNamespaceTargets(modelID string, provider string) []string {
	targetSet := make(map[string]struct{}, 3)
	targets := make([]string, 0, 3)
	if namespace := ExtractNamespaceTarget(modelID); namespace != "" {
		if _, exists := targetSet[namespace]; !exists {
			targetSet[namespace] = struct{}{}
			targets = append(targets, namespace)
		}
	}
	if provider != "" {
		providerLower := strings.ToLower(strings.TrimSpace(provider))
		if _, exists := targetSet[providerLower]; !exists {
			targetSet[providerLower] = struct{}{}
			targets = append(targets, providerLower)
		}
	}
	return targets
}

func EndpointByTarget(endpoints []commontypes.UpstreamConfig, target string) commontypes.UpstreamConfig {
	for _, endpoint := range endpoints {
		if endpoint.URL == target {
			return endpoint
		}
	}
	return commontypes.UpstreamConfig{}
}

func ExtractNamespaceTarget(path string) string {
	normalizedPath := strings.Trim(strings.TrimSpace(path), "/")
	if normalizedPath == "" {
		return ""
	}
	parts := strings.Split(normalizedPath, "/")
	if len(parts) == 0 {
		return ""
	}
	namespace := strings.ToLower(strings.TrimSpace(parts[0]))
	if namespace == "" {
		return ""
	}
	return namespace
}
