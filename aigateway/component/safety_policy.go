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
	CheckChatSensitive(ctx context.Context, model *types.Model, messages []openai.ChatCompletionMessageParamUnion, userUUID string, stream bool) (bool, *rpc.CheckResult, error)
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

func (s *sensitivePolicyImpl) CheckChatSensitive(ctx context.Context, model *types.Model, messages []openai.ChatCompletionMessageParamUnion, userUUID string, stream bool) (bool, *rpc.CheckResult, error) {
	if model == nil || !model.NeedSensitiveCheck || s.moderation == nil {
		return false, nil, nil
	}

	if s.whitelistRule != nil {
		namespaceTargets := BuildNamespaceTargets(model.OfficialName, model.ID)
		rules, err := s.whitelistRule.ListBySensitiveCheckTargets(ctx, namespaceTargets, model.ID)
		if err != nil {
			return false, nil, fmt.Errorf("failed to query white list rules: %w", err)
		}
		if len(rules) != 0 {
			slog.DebugContext(ctx, "Skip Sensitive check with white list", slog.Any("rule", rules[0]))
			return false, nil, nil
		}
	}

	key := fmt.Sprintf("%s:%s", userUUID, model.ID)
	result, err := s.moderation.CheckChatPrompts(ctx, messages, key, stream)
	if err != nil {
		return false, nil, fmt.Errorf("failed to call moderation error:%w", err)
	}
	return true, result, nil
}

func BuildNamespaceTargets(officialName, modelID string) []string {
	targetSet := make(map[string]struct{}, 2)
	targets := make([]string, 0, 2)
	if namespace := ExtractNamespaceTarget(officialName); namespace != "" {
		if _, exists := targetSet[namespace]; !exists {
			targetSet[namespace] = struct{}{}
			targets = append(targets, namespace)
		}
	}
	if namespace := ExtractNamespaceTarget(modelID); namespace != "" {
		if _, exists := targetSet[namespace]; !exists {
			targetSet[namespace] = struct{}{}
			targets = append(targets, namespace)
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
