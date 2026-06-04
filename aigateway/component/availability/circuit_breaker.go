package availability

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"opencsg.com/csghub-server/aigateway/types"
	prom "opencsg.com/csghub-server/builder/prometheus"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
)

// Circuit breaker timing constants
//
// These values are carefully chosen to prevent a deadlock scenario:
//
// Problem: If localCache TTL >= transitionInterval, the following deadlock occurs:
// 1. Circuit opens, localCache stores state=open (TTL=8s)
// 2. After openDuration, transitionWatcher tries open→half_open
// 3. TryTransitionToHalfOpen succeeds in Redis
// 4. getCircuitState reads from localCache (still valid, returns open)
// 5. persistCircuitState writes open back to Redis/DB, overwriting half_open
// 6. Circuit stays open forever (deadlock)
//
// Solution: localCache TTL (4s) < transitionInterval (5s) < Redis TTL (10s)
// - localCache expires before next transition check, forcing fresh Redis read
// - Redis TTL is longest to survive local cache misses
// - transitionInterval is between them to ensure cache is stale when checking
//
// This ensures that when we check for transitions, localCache has expired,
// so we always read the latest state from Redis and don't overwrite it.
const (
	circuitStateCacheTTL      = 10 * time.Second // Redis cache TTL - longest to survive local misses
	circuitStateLocalCacheTTL = 4 * time.Second  // Local cache TTL - shortest to prevent deadlock
	circuitTransitionInterval = 5 * time.Second  // Check interval - between local and Redis TTL
	modelCacheKey             = "aigateway:models"
)

// CircuitBreaker manages circuit breaker state for provider endpoints
type CircuitBreaker interface {
	// Start starts the background circuit transition watcher.
	Start(ctx context.Context) error
	// Stop stops the background circuit transition watcher.
	Stop() error
	// IsAvailable checks if an endpoint is available (not open)
	IsAvailable(ctx context.Context, upstreamID int64) (bool, error)
	// RecordSuccess records a successful request
	RecordSuccess(ctx context.Context, upstreamID int64) error
	// RecordFailure records a failed request
	RecordFailure(ctx context.Context, upstreamID int64, modelID string, err error) error
	// GetCircuitState gets the current circuit state for an endpoint
	GetCircuitState(ctx context.Context, upstreamID int64) (*types.ProviderCircuitStatus, error)
	// GetAllCircuitStates gets all circuit states
	GetAllCircuitStates(ctx context.Context) ([]types.ProviderCircuitStatus, error)
	// ForceOpen forces a circuit to open state
	ForceOpen(ctx context.Context, upstreamID int64, reason string) error
	// ForceClose forces a circuit to close state
	ForceClose(ctx context.Context, upstreamID int64) error
}

type localCircuitCacheEntry struct {
	status    *types.ProviderCircuitStatus
	expiresAt time.Time
}

type circuitBreakerImpl struct {
	config       types.CircuitBreakerConfig
	circuitStore database.AIGatewayUpstreamCircuitStateStore
	stateCache   StateCache
	modelCache   cache.RedisClient

	mu         sync.RWMutex
	localCache map[string]localCircuitCacheEntry

	cancel context.CancelFunc
}

// NewCircuitBreaker creates a new circuit breaker instance
func NewCircuitBreaker(
	config types.CircuitBreakerConfig,
	circuitStore database.AIGatewayUpstreamCircuitStateStore,
	redisClient cache.RedisClient,
) CircuitBreaker {
	return &circuitBreakerImpl{
		config:       config,
		circuitStore: circuitStore,
		stateCache:   NewStateCache(redisClient),
		modelCache:   redisClient,
		localCache:   make(map[string]localCircuitCacheEntry),
	}
}

func (c *circuitBreakerImpl) IsAvailable(ctx context.Context, upstreamID int64) (bool, error) {
	if !c.config.Enabled {
		return true, nil
	}

	state, err := c.getCircuitState(ctx, upstreamID)
	if err != nil {
		// Fail open when state read fails.
		slog.WarnContext(ctx, "Failed to get circuit state, assuming available",
			"error", err,
			"upstream_id", upstreamID)
		return true, nil
	}

	now := time.Now()
	switch state.CircuitState {
	case types.CircuitStateClosed:
		return true, nil
	case types.CircuitStateOpen:
		if state.NextRetryAt != nil && now.After(*state.NextRetryAt) {
			transitioned, transitionErr := c.stateCache.TryTransitionToHalfOpen(ctx, upstreamID, now, circuitStateCacheTTL)
			if transitionErr != nil {
				slog.WarnContext(ctx, "failed to transition circuit to half-open atomically", "error", transitionErr,
					"upstream_id", upstreamID)
				return false, nil
			}
			if transitioned {
				slog.InfoContext(ctx, "circuit breaker transitioned to half-open",
					"upstream_id", upstreamID,
					"old_state", string(types.CircuitStateOpen),
					"new_state", string(types.CircuitStateHalfOpen),
				)
				// Invalidate localCache so the next getCircuitState reads the
				// freshly-transitioned half_open state from Redis instead of
				// returning the stale open state.
				c.invalidateLocalCache(c.localCacheKey(upstreamID))
				updated, getErr := c.getCircuitState(ctx, upstreamID)
				if getErr == nil {
					_ = c.persistCircuitState(ctx, updated)
				}
				return true, nil
			}

			// Also invalidate localCache here so we don't read a stale open state
			// that was cached before another goroutine transitioned to half_open.
			c.invalidateLocalCache(c.localCacheKey(upstreamID))
			latest, getErr := c.getCircuitState(ctx, upstreamID)
			if getErr == nil && latest.CircuitState != types.CircuitStateOpen {
				return true, nil
			}
		}
		return false, nil
	case types.CircuitStateHalfOpen:
		allowed, current, limitErr := c.stateCache.TryAcquireHalfOpenSlot(ctx, upstreamID, c.getHalfOpenMaxRequests(), circuitStateCacheTTL)
		if limitErr != nil {
			slog.WarnContext(ctx, "failed to enforce half-open request limit", "error", limitErr,
				"upstream_id", upstreamID)
			return false, nil
		}
		if !allowed {
			slog.DebugContext(ctx, "half-open request limit reached",
				"upstream_id", upstreamID,
				"current", current, "max", c.getHalfOpenMaxRequests())
			return false, nil
		}
		return true, nil
	default:
		return true, nil
	}
}

func (c *circuitBreakerImpl) RecordSuccess(ctx context.Context, upstreamID int64) error {
	if !c.config.Enabled {
		return nil
	}

	now := time.Now()
	input := types.StateCacheRecordInput{
		UpstreamID: upstreamID,
		Now:        now,
		TTL:        circuitStateCacheTTL,
	}

	if c.stateCache.Enabled() {
		status, err := c.stateCache.RecordSuccess(ctx, input)
		if err == nil {
			return c.persistCircuitState(ctx, status)
		}
		slog.WarnContext(ctx, "failed to record circuit success in redis, fallback to db path", "error", err,
			"upstream_id", upstreamID)
	}

	state, err := c.getCircuitState(ctx, upstreamID)
	if err != nil {
		state = c.newDefaultCircuitStatus(upstreamID, now)
	}
	state.SuccessCount++
	state.FailureCount = 0
	if state.CircuitState == types.CircuitStateHalfOpen {
		slog.InfoContext(ctx, "circuit breaker closed after successful half-open request",
			"upstream_id", upstreamID,
			"old_state", string(types.CircuitStateHalfOpen),
			"new_state", string(types.CircuitStateClosed),
		)
		state.CircuitState = types.CircuitStateClosed
		state.LastStateChange = now
		state.SuccessCount = 0
	}
	state.NextRetryAt = nil
	return c.persistCircuitState(ctx, state)
}

func (c *circuitBreakerImpl) RecordFailure(ctx context.Context, upstreamID int64, modelID string, failErr error) error {
	if !c.config.Enabled {
		return nil
	}

	now := time.Now()
	input := types.StateCacheRecordInput{
		UpstreamID: upstreamID,
		Now:        now,
		TTL:        circuitStateCacheTTL,
	}

	if c.stateCache.Enabled() {
		status, err := c.stateCache.RecordFailure(ctx, input, c.getFailureThreshold(), c.getOpenDuration())
		if err == nil {
			if status.CircuitState == types.CircuitStateOpen {
				slog.InfoContext(ctx, "circuit breaker opened",
					"upstream_id", upstreamID,
					"model_id", modelID,
					"next_retry_at", status.NextRetryAt,
					"error", failErr)
			}
			c.invalidateModelCacheOnOpenAsync(ctx, modelID, upstreamID, status.CircuitState)
			return c.persistCircuitState(ctx, status)
		}
		slog.WarnContext(ctx, "failed to record circuit failure in redis, fallback to db path", "error", err,
			"upstream_id", upstreamID)
	}

	state, err := c.getCircuitState(ctx, upstreamID)
	if err != nil {
		state = c.newDefaultCircuitStatus(upstreamID, now)
	}

	state.FailureCount++
	state.SuccessCount = 0
	if state.CircuitState == types.CircuitStateHalfOpen || state.FailureCount >= c.getFailureThreshold() {
		nextRetry := now.Add(c.getOpenDuration())
		reason := "failure in half-open state"
		if state.CircuitState != types.CircuitStateHalfOpen {
			reason = fmt.Sprintf("consecutive failures (%d) >= threshold (%d)", state.FailureCount, c.getFailureThreshold())
		}
		slog.InfoContext(ctx, "circuit breaker opened",
			"upstream_id", upstreamID,
			"reason", reason, "next_retry_at", nextRetry, "error", failErr)
		state.CircuitState = types.CircuitStateOpen
		state.LastStateChange = now
		state.NextRetryAt = &nextRetry
		state.FailureCount = 0
		state.SuccessCount = 0
	}

	c.invalidateModelCacheOnOpenAsync(ctx, modelID, upstreamID, state.CircuitState)
	return c.persistCircuitState(ctx, state)
}

func (c *circuitBreakerImpl) invalidateModelCacheOnOpenAsync(ctx context.Context, modelID string, upstreamID int64, state types.CircuitState) {
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		c.invalidateModelCacheOnOpen(cacheCtx, modelID, upstreamID, state)
	}()
}

func (c *circuitBreakerImpl) invalidateModelCacheOnOpen(ctx context.Context, modelID string, upstreamID int64, state types.CircuitState) {
	if state != types.CircuitStateOpen || c.modelCache == nil {
		return
	}
	cacheModelID := strings.TrimSpace(modelID)
	if cacheModelID == "" {
		return
	}
	if err := c.modelCache.HDel(ctx, modelCacheKey, cacheModelID); err != nil {
		slog.WarnContext(ctx, "failed to invalidate model cache after circuit opened",
			"model_id", cacheModelID, "upstream_id", upstreamID, "error", err)
		return
	}
	slog.InfoContext(ctx, "invalidated model cache after circuit opened",
		"model_id", cacheModelID, "upstream_id", upstreamID)
}

func (c *circuitBreakerImpl) GetCircuitState(ctx context.Context, upstreamID int64) (*types.ProviderCircuitStatus, error) {
	return c.getCircuitState(ctx, upstreamID)
}

func (c *circuitBreakerImpl) GetAllCircuitStates(ctx context.Context) ([]types.ProviderCircuitStatus, error) {
	openStates, err := c.circuitStore.GetAllOpen(ctx)
	if err != nil {
		return nil, err
	}
	closedStates, err := c.circuitStore.GetAllClosed(ctx)
	if err != nil {
		return nil, err
	}
	results := make([]types.ProviderCircuitStatus, 0, len(openStates)+len(closedStates))
	for _, state := range openStates {
		results = append(results, convertCircuitDBState(&state))
	}
	for _, state := range closedStates {
		results = append(results, convertCircuitDBState(&state))
	}
	return results, nil
}

func (c *circuitBreakerImpl) ForceOpen(ctx context.Context, upstreamID int64, reason string) error {
	state, err := c.getCircuitState(ctx, upstreamID)
	if err != nil {
		state = c.newDefaultCircuitStatus(upstreamID, time.Now())
	}
	if state.CircuitState == types.CircuitStateOpen {
		slog.DebugContext(ctx, "circuit breaker already open, skipping persist", "upstream_id", upstreamID)
		return nil
	}
	now := time.Now()
	nextRetry := now.Add(c.getOpenDuration())
	state.CircuitState = types.CircuitStateOpen
	state.LastStateChange = now
	state.NextRetryAt = &nextRetry
	state.FailureCount = 0
	state.SuccessCount = 0
	slog.InfoContext(ctx, "circuit breaker force opened", "upstream_id", upstreamID, "reason", reason)
	return c.persistCircuitState(ctx, state)
}

func (c *circuitBreakerImpl) ForceClose(ctx context.Context, upstreamID int64) error {
	state, err := c.getCircuitState(ctx, upstreamID)
	if err != nil {
		state = c.newDefaultCircuitStatus(upstreamID, time.Now())
	}
	if state.CircuitState == types.CircuitStateClosed && state.FailureCount == 0 && state.NextRetryAt == nil {
		slog.DebugContext(ctx, "circuit breaker already closed, skipping persist", "upstream_id", upstreamID)
		return nil
	}
	state.CircuitState = types.CircuitStateClosed
	state.LastStateChange = time.Now()
	state.NextRetryAt = nil
	state.FailureCount = 0
	state.SuccessCount = 0
	slog.InfoContext(ctx, "circuit breaker force closed", "upstream_id", upstreamID)
	return c.persistCircuitState(ctx, state)
}

func (c *circuitBreakerImpl) Start(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}
	watcherCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	go c.runTransitionWatcher(watcherCtx)
	slog.InfoContext(ctx, "circuit breaker transition watcher started",
		"interval", circuitTransitionInterval)
	return nil
}

func (c *circuitBreakerImpl) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

// runTransitionWatcher periodically scans open circuits and transitions
// eligible ones to half-open when their retry window has elapsed.
func (c *circuitBreakerImpl) runTransitionWatcher(ctx context.Context) {
	ticker := time.NewTicker(circuitTransitionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkAndTransitionOpenCircuits(ctx)
		}
	}
}

// checkAndTransitionOpenCircuits queries all open circuits from DB and
// transitions those whose NextRetryAt has passed to half-open state.
func (c *circuitBreakerImpl) checkAndTransitionOpenCircuits(ctx context.Context) {
	openStates, err := c.circuitStore.GetAllOpen(ctx)
	if err != nil {
		slog.WarnContext(ctx, "failed to query open circuits for transition check", "error", err)
		return
	}
	if len(openStates) == 0 {
		return
	}

	now := time.Now()
	for _, dbState := range openStates {
		if dbState.NextRetryAt == nil || !now.After(*dbState.NextRetryAt) {
			continue
		}

		status := convertCircuitDBState(&dbState)
		if err := c.stateCache.SetCircuitState(ctx, &status, circuitStateCacheTTL); err != nil {
			slog.WarnContext(ctx, "failed to set circuit state after transition",
				"error", err,
				"upstream_id", dbState.UpstreamID)
			continue
		}

		transitioned, err := c.stateCache.TryTransitionToHalfOpen(ctx, dbState.UpstreamID, now, circuitStateCacheTTL)
		if err != nil {
			slog.WarnContext(ctx, "failed to transition circuit to half-open",
				"error", err,
				"upstream_id", dbState.UpstreamID)
			continue
		}
		if !transitioned {
			continue
		}
		slog.InfoContext(ctx, "circuit breaker transitioned to half-open by watcher",
			"upstream_id", dbState.UpstreamID,
			"old_state", string(types.CircuitStateOpen),
			"new_state", string(types.CircuitStateHalfOpen),
		)
		// Invalidate localCache so getCircuitState reads the freshly-transitioned
		// half_open state from Redis instead of returning the stale open state.
		c.invalidateLocalCache(c.localCacheKey(dbState.UpstreamID))
		updated, getErr := c.getCircuitState(ctx, dbState.UpstreamID)
		if getErr != nil {
			slog.WarnContext(ctx, "failed to read circuit state after half-open transition",
				"error", getErr,
				"upstream_id", dbState.UpstreamID)
			continue
		}
		if err := c.persistCircuitState(ctx, updated); err != nil {
			slog.WarnContext(ctx, "failed to persist half-open circuit state",
				"error", err,
				"upstream_id", dbState.UpstreamID)
		}
	}
}

func (c *circuitBreakerImpl) getCircuitState(ctx context.Context, upstreamID int64) (*types.ProviderCircuitStatus, error) {
	cacheKey := c.localCacheKey(upstreamID)
	if cached := c.getLocalCircuitCache(cacheKey); cached != nil {
		return cached, nil
	}

	if c.stateCache.Enabled() {
		cached, err := c.stateCache.GetCircuitState(ctx, upstreamID)
		if err == nil {
			c.setLocalCircuitCache(cacheKey, cached)
			return cached, nil
		}
	}

	state, err := c.circuitStore.GetByUpstreamID(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	status := convertCircuitDBState(state)
	c.setLocalCircuitCache(cacheKey, &status)
	if err := c.stateCache.SetCircuitState(ctx, &status, circuitStateCacheTTL); err != nil {
		slog.ErrorContext(ctx, "failed to set circuit state in get circuit state",
			"error", err,
			"upstream_id", upstreamID)
	}
	return &status, nil
}

func (c *circuitBreakerImpl) persistCircuitState(ctx context.Context, status *types.ProviderCircuitStatus) error {
	if status == nil {
		return nil
	}
	dbState := &database.AIGatewayUpstreamCircuitState{
		UpstreamID:      status.UpstreamID,
		CircuitState:    string(status.CircuitState),
		FailureCount:    status.FailureCount,
		SuccessCount:    status.SuccessCount,
		LastStateChange: status.LastStateChange,
		NextRetryAt:     status.NextRetryAt,
	}
	if err := c.circuitStore.Upsert(ctx, dbState); err != nil {
		return err
	}
	if err := c.stateCache.SetCircuitState(ctx, status, circuitStateCacheTTL); err != nil {
		slog.ErrorContext(ctx, "failed to set circuit state in persist circuit state",
			"error", err,
			"upstream_id", status.UpstreamID)
	}
	c.setLocalCircuitCache(c.localCacheKey(status.UpstreamID), status)
	// Report metric
	if prom.AIGatewayUpstreamCircuitState != nil {
		prom.AIGatewayUpstreamCircuitState.WithLabelValues(
			strconv.FormatInt(status.UpstreamID, 10),
			status.ModelName,
			status.Provider,
			string(status.CircuitState),
		).Set(circuitStateToGaugeValue(status.CircuitState))
	}
	return nil
}

func circuitStateToGaugeValue(state types.CircuitState) float64 {
	switch state {
	case types.CircuitStateOpen:
		return 0
	case types.CircuitStateHalfOpen:
		return 1
	default:
		return 2
	}
}

func (c *circuitBreakerImpl) newDefaultCircuitStatus(upstreamID int64, now time.Time) *types.ProviderCircuitStatus {
	return &types.ProviderCircuitStatus{
		UpstreamID:      upstreamID,
		CircuitState:    types.CircuitStateClosed,
		FailureCount:    0,
		SuccessCount:    0,
		LastStateChange: now,
		NextRetryAt:     nil,
	}
}

func (c *circuitBreakerImpl) getLocalCircuitCache(cacheKey string) *types.ProviderCircuitStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.localCache[cacheKey]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}
	copyStatus := *entry.status
	return &copyStatus
}

func (c *circuitBreakerImpl) setLocalCircuitCache(cacheKey string, status *types.ProviderCircuitStatus) {
	if status == nil {
		return
	}
	copyStatus := *status
	c.mu.Lock()
	defer c.mu.Unlock()
	c.localCache[cacheKey] = localCircuitCacheEntry{
		status:    &copyStatus,
		expiresAt: time.Now().Add(circuitStateLocalCacheTTL),
	}
}

func (c *circuitBreakerImpl) invalidateLocalCache(cacheKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.localCache, cacheKey)
}

func (c *circuitBreakerImpl) localCacheKey(upstreamID int64) string {
	return strconv.FormatInt(upstreamID, 10)
}

func (c *circuitBreakerImpl) getFailureThreshold() int {
	if c.config.FailureThreshold <= 0 {
		return 3
	}
	return c.config.FailureThreshold
}

func (c *circuitBreakerImpl) getOpenDuration() time.Duration {
	if c.config.OpenDuration <= 0 {
		return 30 * time.Second
	}
	return c.config.OpenDuration
}

func (c *circuitBreakerImpl) getHalfOpenMaxRequests() int {
	if c.config.HalfOpenMaxRequests <= 0 {
		return 1
	}
	return c.config.HalfOpenMaxRequests
}

func convertCircuitDBState(state *database.AIGatewayUpstreamCircuitState) types.ProviderCircuitStatus {
	return types.ProviderCircuitStatus{
		UpstreamID:      state.UpstreamID,
		CircuitState:    types.CircuitState(state.CircuitState),
		FailureCount:    state.FailureCount,
		SuccessCount:    state.SuccessCount,
		LastStateChange: state.LastStateChange,
		NextRetryAt:     state.NextRetryAt,
	}
}
