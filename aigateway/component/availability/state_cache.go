package availability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/cache"
)

const (
	stateCacheKeyPrefix                 = "aigateway:availability"
	stateCacheDefaultCircuitTTL         = 30 * time.Second
	stateCacheDefaultHealthTTL          = 30 * time.Second
	stateCacheDefaultHalfOpenCounterTTL = 30 * time.Second
)

const transitionToHalfOpenScript = `
local state = redis.call('HGET', KEYS[1], 'circuit_state')
if not state or state ~= 'open' then
	return 0
end
local next_retry_at = tonumber(redis.call('HGET', KEYS[1], 'next_retry_at') or '0')
if next_retry_at == 0 then
	return 0
end
if next_retry_at > tonumber(ARGV[1]) then
	return 0
end
redis.call('HSET', KEYS[1], 'circuit_state', 'half_open')
redis.call('HSET', KEYS[1], 'failure_count', 0)
redis.call('HSET', KEYS[1], 'success_count', 0)
redis.call('HSET', KEYS[1], 'last_state_change', ARGV[1])
redis.call('HDEL', KEYS[1], 'next_retry_at')
redis.call('EXPIRE', KEYS[1], ARGV[2])
redis.call('DEL', KEYS[2])
return 1
`

const incrementHalfOpenRequestsScript = `
local current = tonumber(redis.call('GET', KEYS[1]) or '0')
local limit = tonumber(ARGV[1])
if current >= limit then
	return {0, current}
end
current = redis.call('INCR', KEYS[1])
redis.call('EXPIRE', KEYS[1], ARGV[2])
return {1, current}
`

const recordFailureScript = `
local state = redis.call('HGET', KEYS[1], 'circuit_state')
if not state then
	state = 'closed'
end
local failure = tonumber(redis.call('HGET', KEYS[1], 'failure_count') or '0') + 1
local success = 0
local now_ts = tonumber(ARGV[3])
local threshold = tonumber(ARGV[1])
local open_duration = tonumber(ARGV[2])
local next_retry_at = 0

if state == 'half_open' or failure >= threshold then
	state = 'open'
	failure = 0
	next_retry_at = now_ts + open_duration
	redis.call('DEL', KEYS[2])
end

redis.call('HSET', KEYS[1], 'circuit_state', state)
redis.call('HSET', KEYS[1], 'failure_count', failure)
redis.call('HSET', KEYS[1], 'success_count', success)
redis.call('HSET', KEYS[1], 'last_state_change', now_ts)
if next_retry_at > 0 then
	redis.call('HSET', KEYS[1], 'next_retry_at', next_retry_at)
else
	redis.call('HDEL', KEYS[1], 'next_retry_at')
end
redis.call('EXPIRE', KEYS[1], ARGV[4])

return {state, failure, success, now_ts, next_retry_at}
`

const recordSuccessScript = `
local state = redis.call('HGET', KEYS[1], 'circuit_state')
if not state then
	state = 'closed'
end
local failure = 0
local success = tonumber(redis.call('HGET', KEYS[1], 'success_count') or '0') + 1
local now_ts = tonumber(ARGV[1])

if state == 'half_open' then
	state = 'closed'
	success = 0
	redis.call('DEL', KEYS[2])
end

redis.call('HSET', KEYS[1], 'circuit_state', state)
redis.call('HSET', KEYS[1], 'failure_count', failure)
redis.call('HSET', KEYS[1], 'success_count', success)
redis.call('HSET', KEYS[1], 'last_state_change', now_ts)
redis.call('HDEL', KEYS[1], 'next_retry_at')
redis.call('EXPIRE', KEYS[1], ARGV[2])

return {state, failure, success, now_ts, 0}
`

const renewLeaderScript = `
local current = redis.call('GET', KEYS[1])
if current ~= ARGV[1] then
	return 0
end
redis.call('EXPIRE', KEYS[1], ARGV[2])
return 1
`

var errStateCacheMiss = errors.New("state cache miss")

type stateCacheRecordInput struct {
	UpstreamID int64
	Now        time.Time
	TTL        time.Duration
}

type StateCache interface {
	Enabled() bool
	GetCircuitState(ctx context.Context, upstreamID int64) (*types.ProviderCircuitStatus, error)
	SetCircuitState(ctx context.Context, state *types.ProviderCircuitStatus, ttl time.Duration) error
	TryTransitionToHalfOpen(ctx context.Context, upstreamID int64, now time.Time, ttl time.Duration) (bool, error)
	TryAcquireHalfOpenSlot(ctx context.Context, upstreamID int64, maxRequests int, ttl time.Duration) (bool, int64, error)
	RecordFailure(ctx context.Context, input stateCacheRecordInput, failureThreshold int, openDuration time.Duration) (*types.ProviderCircuitStatus, error)
	RecordSuccess(ctx context.Context, input stateCacheRecordInput) (*types.ProviderCircuitStatus, error)
	GetHealthState(ctx context.Context, upstreamID int64) (*types.ProviderHealthStatus, error)
	SetHealthState(ctx context.Context, state *types.ProviderHealthStatus, ttl time.Duration) error
	TryAcquireLeader(ctx context.Context, electionKey, ownerID string, ttl time.Duration) (bool, error)
	RenewLeader(ctx context.Context, electionKey, ownerID string, ttl time.Duration) (bool, error)
	GetLeader(ctx context.Context, electionKey string) (string, error)
}

type stateCacheImpl struct {
	redisClient cache.RedisClient
}

func NewStateCache(redisClient cache.RedisClient) StateCache {
	return &stateCacheImpl{
		redisClient: redisClient,
	}
}

func (s *stateCacheImpl) Enabled() bool {
	return s != nil && s.redisClient != nil
}

func (s *stateCacheImpl) GetCircuitState(ctx context.Context, upstreamID int64) (*types.ProviderCircuitStatus, error) {
	if !s.Enabled() {
		return nil, errStateCacheMiss
	}
	fields, err := s.redisClient.HGetAll(ctx, s.circuitStateKey(upstreamID))
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, errStateCacheMiss
	}

	failureCount, _ := strconv.Atoi(fields["failure_count"])
	successCount, _ := strconv.Atoi(fields["success_count"])
	lastStateChangeUnix, _ := strconv.ParseInt(fields["last_state_change"], 10, 64)
	nextRetryAtUnix, _ := strconv.ParseInt(fields["next_retry_at"], 10, 64)
	var nextRetryAt *time.Time
	if nextRetryAtUnix > 0 {
		next := time.Unix(nextRetryAtUnix, 0)
		nextRetryAt = &next
	}

	return &types.ProviderCircuitStatus{
		UpstreamID:      upstreamID,
		CircuitState:    types.CircuitState(fields["circuit_state"]),
		FailureCount:    failureCount,
		SuccessCount:    successCount,
		LastStateChange: time.Unix(lastStateChangeUnix, 0),
		NextRetryAt:     nextRetryAt,
	}, nil
}

func (s *stateCacheImpl) SetCircuitState(ctx context.Context, state *types.ProviderCircuitStatus, ttl time.Duration) error {
	if !s.Enabled() || state == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = stateCacheDefaultCircuitTTL
	}

	args := []any{
		"circuit_state", string(state.CircuitState),
		"failure_count", state.FailureCount,
		"success_count", state.SuccessCount,
		"last_state_change", state.LastStateChange.Unix(),
	}
	if state.NextRetryAt != nil {
		args = append(args, "next_retry_at", state.NextRetryAt.Unix())
	}
	circuitKey := s.circuitStateKey(state.UpstreamID)
	if err := s.redisClient.HMSet(ctx, circuitKey, args...); err != nil {
		return err
	}
	if state.NextRetryAt == nil {
		if err := s.redisClient.HDel(ctx, circuitKey, "next_retry_at"); err != nil {
			return err
		}
	}
	return s.redisClient.Expire(ctx, circuitKey, ttl)
}

func (s *stateCacheImpl) TryTransitionToHalfOpen(ctx context.Context, upstreamID int64, now time.Time, ttl time.Duration) (bool, error) {
	if !s.Enabled() {
		return false, nil
	}
	if ttl <= 0 {
		ttl = stateCacheDefaultCircuitTTL
	}
	result, err := s.redisClient.RunScript(
		ctx,
		transitionToHalfOpenScript,
		[]string{s.circuitStateKey(upstreamID), s.halfOpenCounterKey(upstreamID)},
		now.Unix(),
		int(ttl.Seconds()),
	)
	if err != nil {
		return false, err
	}
	success, err := scriptResultToInt64(result)
	if err != nil {
		return false, err
	}
	return success == 1, nil
}

func (s *stateCacheImpl) TryAcquireHalfOpenSlot(ctx context.Context, upstreamID int64, maxRequests int, ttl time.Duration) (bool, int64, error) {
	if !s.Enabled() {
		return true, 1, nil
	}
	if maxRequests <= 0 {
		maxRequests = 1
	}
	if ttl <= 0 {
		ttl = stateCacheDefaultHalfOpenCounterTTL
	}
	result, err := s.redisClient.RunScript(
		ctx,
		incrementHalfOpenRequestsScript,
		[]string{s.halfOpenCounterKey(upstreamID)},
		maxRequests,
		int(ttl.Seconds()),
	)
	if err != nil {
		return false, 0, err
	}
	values, ok := result.([]any)
	if !ok || len(values) != 2 {
		return false, 0, fmt.Errorf("invalid half-open script result type: %T", result)
	}
	allowed, err := scriptResultToInt64(values[0])
	if err != nil {
		return false, 0, err
	}
	current, err := scriptResultToInt64(values[1])
	if err != nil {
		return false, 0, err
	}
	return allowed == 1, current, nil
}

func (s *stateCacheImpl) RecordFailure(ctx context.Context, input stateCacheRecordInput, failureThreshold int, openDuration time.Duration) (*types.ProviderCircuitStatus, error) {
	if !s.Enabled() {
		return nil, errStateCacheMiss
	}
	if failureThreshold <= 0 {
		failureThreshold = 3
	}
	if openDuration <= 0 {
		openDuration = 30 * time.Second
	}
	ttlSeconds := int(input.TTL.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = int(stateCacheDefaultCircuitTTL.Seconds())
	}

	result, err := s.redisClient.RunScript(
		ctx,
		recordFailureScript,
		[]string{
			s.circuitStateKey(input.UpstreamID),
			s.halfOpenCounterKey(input.UpstreamID),
		},
		failureThreshold,
		int(openDuration.Seconds()),
		input.Now.Unix(),
		ttlSeconds,
	)
	if err != nil {
		return nil, err
	}
	return s.parseCircuitScriptResult(input, result)
}

func (s *stateCacheImpl) RecordSuccess(ctx context.Context, input stateCacheRecordInput) (*types.ProviderCircuitStatus, error) {
	if !s.Enabled() {
		return nil, errStateCacheMiss
	}
	ttlSeconds := int(input.TTL.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = int(stateCacheDefaultCircuitTTL.Seconds())
	}
	result, err := s.redisClient.RunScript(
		ctx,
		recordSuccessScript,
		[]string{
			s.circuitStateKey(input.UpstreamID),
			s.halfOpenCounterKey(input.UpstreamID),
		},
		input.Now.Unix(),
		ttlSeconds,
	)
	if err != nil {
		return nil, err
	}
	return s.parseCircuitScriptResult(input, result)
}

func (s *stateCacheImpl) GetHealthState(ctx context.Context, upstreamID int64) (*types.ProviderHealthStatus, error) {
	if !s.Enabled() {
		return nil, errStateCacheMiss
	}
	value, err := s.redisClient.Get(ctx, s.healthStateKey(upstreamID))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(value) == "" {
		return nil, errStateCacheMiss
	}

	var status types.ProviderHealthStatus
	if err := json.Unmarshal([]byte(value), &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (s *stateCacheImpl) SetHealthState(ctx context.Context, state *types.ProviderHealthStatus, ttl time.Duration) error {
	if !s.Enabled() || state == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = stateCacheDefaultHealthTTL
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.redisClient.SetEx(ctx, s.healthStateKey(state.UpstreamID), string(payload), ttl)
}

func (s *stateCacheImpl) TryAcquireLeader(ctx context.Context, electionKey, ownerID string, ttl time.Duration) (bool, error) {
	if !s.Enabled() {
		return true, nil
	}
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	return s.redisClient.SetNX(ctx, s.leaderKey(electionKey), ownerID, ttl)
}

func (s *stateCacheImpl) RenewLeader(ctx context.Context, electionKey, ownerID string, ttl time.Duration) (bool, error) {
	if !s.Enabled() {
		return true, nil
	}
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	result, err := s.redisClient.RunScript(
		ctx,
		renewLeaderScript,
		[]string{s.leaderKey(electionKey)},
		ownerID,
		int(ttl.Seconds()),
	)
	if err != nil {
		return false, err
	}
	renewed, err := scriptResultToInt64(result)
	if err != nil {
		return false, err
	}
	return renewed == 1, nil
}

func (s *stateCacheImpl) GetLeader(ctx context.Context, electionKey string) (string, error) {
	if !s.Enabled() {
		return "", errStateCacheMiss
	}
	return s.redisClient.Get(ctx, s.leaderKey(electionKey))
}


func (s *stateCacheImpl) parseCircuitScriptResult(input stateCacheRecordInput, result any) (*types.ProviderCircuitStatus, error) {
	values, ok := result.([]any)
	if !ok || len(values) != 5 {
		return nil, fmt.Errorf("invalid circuit script result type: %T", result)
	}

	state, ok := values[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid state value type: %T", values[0])
	}
	failureCount, err := scriptResultToInt64(values[1])
	if err != nil {
		return nil, err
	}
	successCount, err := scriptResultToInt64(values[2])
	if err != nil {
		return nil, err
	}
	lastStateChangeUnix, err := scriptResultToInt64(values[3])
	if err != nil {
		return nil, err
	}
	nextRetryAtUnix, err := scriptResultToInt64(values[4])
	if err != nil {
		return nil, err
	}

	var nextRetryAt *time.Time
	if nextRetryAtUnix > 0 {
		next := time.Unix(nextRetryAtUnix, 0)
		nextRetryAt = &next
	}
	return &types.ProviderCircuitStatus{
		UpstreamID:      input.UpstreamID,
		Provider:        strconv.FormatInt(input.UpstreamID, 10),
		ModelName:       "",
		Endpoint:        "",
		CircuitState:    types.CircuitState(state),
		FailureCount:    int(failureCount),
		SuccessCount:    int(successCount),
		LastStateChange: time.Unix(lastStateChangeUnix, 0),
		NextRetryAt:     nextRetryAt,
	}, nil
}

func (s *stateCacheImpl) circuitStateKey(upstreamID int64) string {
	return fmt.Sprintf("%s:circuit:%d", stateCacheKeyPrefix, upstreamID)
}

func (s *stateCacheImpl) halfOpenCounterKey(upstreamID int64) string {
	return fmt.Sprintf("%s:circuit:half-open:%d", stateCacheKeyPrefix, upstreamID)
}

func (s *stateCacheImpl) healthStateKey(upstreamID int64) string {
	return fmt.Sprintf("%s:health:%d", stateCacheKeyPrefix, upstreamID)
}

func (s *stateCacheImpl) leaderKey(electionKey string) string {
	return fmt.Sprintf("%s:leader:%s", stateCacheKeyPrefix, electionKey)
}

func scriptResultToInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case []byte:
		parsed, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse int64 from []byte: %w", err)
		}
		return parsed, nil
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse int64 from string: %w", err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported numeric type: %T", value)
	}
}
