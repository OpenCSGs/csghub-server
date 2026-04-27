package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/cache"
	commontypes "opencsg.com/csghub-server/common/types"
)

const (
	defaultUsageLimitWindowSeconds = int64(60)
	usageLimitKeyPrefix            = "aigateway:usage"
	usageLimitTTLBufferSeconds     = int64(60)
)

const usageLimitCheckScript = `
local total = tonumber(redis.call('HGET', KEYS[1], 'total') or '0')
local prompt = tonumber(redis.call('HGET', KEYS[1], 'prompt') or '0')
local completion = tonumber(redis.call('HGET', KEYS[1], 'completion') or '0')
local max_total = tonumber(ARGV[1] or '0')
local max_prompt = tonumber(ARGV[2] or '0')
local max_completion = tonumber(ARGV[3] or '0')

if max_total > 0 and total >= max_total then
	return 0
end
if max_prompt > 0 and prompt >= max_prompt then
	return 0
end
if max_completion > 0 and completion >= max_completion then
	return 0
end
return 1
`

const usageLimitCommitScript = `
redis.call('HINCRBY', KEYS[1], 'total', ARGV[1])
redis.call('HINCRBY', KEYS[1], 'prompt', ARGV[2])
redis.call('HINCRBY', KEYS[1], 'completion', ARGV[3])
redis.call('EXPIRE', KEYS[1], ARGV[4])
return 1
`

type UsageLimiter interface {
	Check(ctx context.Context, userUUID string, model *types.Model, endpoint string) error
	Commit(ctx context.Context, userUUID string, model *types.Model, endpoint string, usage *token.Usage) error
}

type UsageLimitExceededError struct {
	Message           string
	RetryAfterSeconds int64
}

func (e *UsageLimitExceededError) Error() string {
	if e == nil || e.Message == "" {
		return "usage limit exceeded"
	}
	return e.Message
}

func IsUsageLimitExceeded(err error) bool {
	var usageErr *UsageLimitExceededError
	return errors.As(err, &usageErr)
}

type usageLimiterImpl struct {
	redisClient cache.RedisClient
	nowFn       func() time.Time
}

func NewUsageLimiter(redisClient cache.RedisClient) UsageLimiter {
	return &usageLimiterImpl{
		redisClient: redisClient,
		nowFn:       time.Now,
	}
}

func (l *usageLimiterImpl) Check(ctx context.Context, userUUID string, model *types.Model, endpoint string) error {
	policy := getUsageLimitPolicy(model, endpoint)
	if policy == nil || strings.TrimSpace(userUUID) == "" || l.redisClient == nil {
		return nil
	}

	now := l.nowFn()
	key := buildUsageLimitKey(userUUID, model, endpoint, policy.WindowSeconds, now)
	result, err := l.redisClient.RunScript(
		ctx,
		usageLimitCheckScript,
		[]string{key},
		policy.MaxTotalTokens,
		policy.MaxPromptTokens,
		policy.MaxCompletionTokens,
	)
	if err != nil {
		slog.WarnContext(ctx, "usage limit check failed, fallback to allow", slog.Any("error", err), slog.String("key", key))
		return nil
	}
	allowed, convErr := scriptResultToInt64(result)
	if convErr != nil {
		slog.WarnContext(ctx, "usage limit check returned unexpected result, fallback to allow", slog.Any("error", convErr), slog.Any("result", result))
		return nil
	}
	if allowed == 1 {
		return nil
	}
	return &UsageLimitExceededError{
		Message:           "usage quota exceeded for current window",
		RetryAfterSeconds: secondsUntilWindowEnd(policy.WindowSeconds, now),
	}
}

func (l *usageLimiterImpl) Commit(ctx context.Context, userUUID string, model *types.Model, endpoint string, usage *token.Usage) error {
	policy := getUsageLimitPolicy(model, endpoint)
	if policy == nil || strings.TrimSpace(userUUID) == "" || usage == nil || l.redisClient == nil {
		return nil
	}

	promptCost, completionCost, totalCost := normalizeUsageForLimit(usage, *policy)
	if promptCost <= 0 && completionCost <= 0 && totalCost <= 0 {
		return nil
	}

	now := l.nowFn()
	key := buildUsageLimitKey(userUUID, model, endpoint, policy.WindowSeconds, now)
	ttlSeconds := secondsUntilWindowEnd(policy.WindowSeconds, now) + usageLimitTTLBufferSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = policy.WindowSeconds
	}
	if _, err := l.redisClient.RunScript(
		ctx,
		usageLimitCommitScript,
		[]string{key},
		totalCost,
		promptCost,
		completionCost,
		ttlSeconds,
	); err != nil {
		slog.WarnContext(ctx, "usage limit commit failed, ignoring to preserve response path", slog.Any("error", err), slog.String("key", key))
	}
	return nil
}

func (m *openaiComponentImpl) CheckUsageLimit(ctx context.Context, userUUID string, model *types.Model, endpoint string) error {
	return m.getUsageLimiter().Check(ctx, userUUID, model, endpoint)
}

func (m *openaiComponentImpl) CommitUsageLimit(ctx context.Context, userUUID string, model *types.Model, tokenCounter token.Counter) error {
	if tokenCounter == nil {
		return nil
	}
	usage, err := tokenCounter.Usage(ctx)
	if err != nil {
		return err
	}
	return m.getUsageLimiter().Commit(ctx, userUUID, model, model.Endpoint, usage)
}

func getUsageLimitPolicy(model *types.Model, endpoint string) *commontypes.UsageLimitPolicy {
	if model == nil {
		return nil
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(model.Endpoint)
	}
	if endpoint == "" {
		return nil
	}
	for _, ep := range model.Upstreams {
		if strings.TrimSpace(ep.URL) != endpoint {
			continue
		}
		if ep.LimitPolicy == nil || !ep.LimitPolicy.Enabled {
			return nil
		}
		policy := *ep.LimitPolicy
		if policy.WindowSeconds <= 0 {
			policy.WindowSeconds = defaultUsageLimitWindowSeconds
		}
		return &policy
	}
	return nil
}

func buildUsageLimitKey(userUUID string, model *types.Model, endpoint string, windowSeconds int64, now time.Time) string {
	if windowSeconds <= 0 {
		windowSeconds = defaultUsageLimitWindowSeconds
	}
	windowStart := now.Unix() / windowSeconds * windowSeconds
	provider := model.OwnedBy
	if strings.TrimSpace(model.Provider) != "" {
		provider = model.Provider
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = strings.TrimSpace(model.Endpoint)
	}
	if endpoint == "" {
		endpoint = "default"
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s:%d", usageLimitKeyPrefix, userUUID, provider, model.ID, endpoint, windowStart)
}

func secondsUntilWindowEnd(windowSeconds int64, now time.Time) int64 {
	if windowSeconds <= 0 {
		windowSeconds = defaultUsageLimitWindowSeconds
	}
	nextWindowStart := (now.Unix()/windowSeconds + 1) * windowSeconds
	retryAfter := nextWindowStart - now.Unix()
	if retryAfter <= 0 {
		return windowSeconds
	}
	return retryAfter
}

func normalizeUsageForLimit(usage *token.Usage, policy commontypes.UsageLimitPolicy) (promptCost int64, completionCost int64, totalCost int64) {
	if usage == nil {
		return 0, 0, 0
	}

	cachedRatio := policy.CachedTokenCostRatio
	cacheCreateRatio := policy.CacheCreateCostRatio
	if cachedRatio < 0 {
		cachedRatio = 0
	}
	if cacheCreateRatio <= 0 {
		cacheCreateRatio = 1
	}

	nonCachedPrompt := usage.PromptTokens - usage.CachedPromptTokens
	if nonCachedPrompt < 0 {
		nonCachedPrompt = 0
	}

	promptCostFloat := float64(nonCachedPrompt) +
		float64(usage.CachedPromptTokens)*cachedRatio +
		float64(usage.CacheCreationPromptTokens)*cacheCreateRatio
	promptCost = int64(math.Ceil(promptCostFloat))
	completionCost = usage.CompletionTokens
	totalCost = promptCost + completionCost
	return
}

func scriptResultToInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected script result type %T", value)
	}
}
