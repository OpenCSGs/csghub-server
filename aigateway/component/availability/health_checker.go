package availability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"opencsg.com/csghub-server/aigateway/types"
	prom "opencsg.com/csghub-server/builder/prometheus"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

const (
	healthStateCacheTTL     = 30 * time.Second
	healthLeaderElectionKey = "health-checker"
	healthLeaderTTL         = 15 * time.Second
	healthLeaderHeartbeat   = 5 * time.Second
)

type HealthChecker interface {
	Start(ctx context.Context) error
	Stop() error
	CheckNow(ctx context.Context, upstreamID int64) (*types.HealthCheckResult, error)
	GetHealthState(ctx context.Context, upstreamID int64) (*types.ProviderHealthStatus, error)
	GetAllHealthStates(ctx context.Context) ([]types.ProviderHealthStatus, error)
}

type HealthCheckerConfig struct {
	Config types.HealthCheckConfig
}

type healthCheckerImpl struct {
	circuitBreaker CircuitBreaker
	config         HealthCheckerConfig
	healthStore    database.AIGatewayUpstreamHealthStateStore
	upstreamStore  database.UpstreamStore
	stateCache     StateCache
	httpClient     *http.Client
	stopCh         chan struct{}
	wg             sync.WaitGroup
	leaderNodeID   string
	isLeader       atomic.Bool
	lastSeenLeader string
}

func NewHealthChecker(
	circuitBreaker CircuitBreaker,
	cfg *config.Config,
	healthStore database.AIGatewayUpstreamHealthStateStore,
	upstreamStore database.UpstreamStore,
	redisClient cache.RedisClient,
) HealthChecker {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "aigateway"
	}

	healthConfig := HealthCheckerConfig{
		Config: types.HealthCheckConfig{
			Enabled: cfg.AIGateway.HealthCheckEnabled,
			L7APICheck: types.L7APICheckConfig{
				Enabled:  cfg.AIGateway.HealthCheckL7APIEnabled,
				Interval: time.Duration(cfg.AIGateway.HealthCheckL7APIInterval) * time.Second,
				Timeout:  time.Duration(cfg.AIGateway.HealthCheckL7APITimeout) * time.Second,
			},
			InferenceCheck: types.InferenceCheckConfig{
				Enabled: false,
			},
			HealthRules: types.HealthRulesConfig{
				ConsecutiveFailuresForUnhealthy: cfg.AIGateway.HealthCheckConsecutiveFailures,
				LatencyThresholdForDegraded:     time.Duration(cfg.AIGateway.HealthCheckLatencyDegradedMs) * time.Millisecond,
			},
		},
	}

	if healthConfig.Config.HealthRules.ConsecutiveFailuresForUnhealthy <= 0 {
		healthConfig.Config.HealthRules.ConsecutiveFailuresForUnhealthy = 3
	}
	if healthConfig.Config.L7APICheck.Interval <= 0 {
		healthConfig.Config.L7APICheck.Interval = 10 * time.Second
	}
	if healthConfig.Config.L7APICheck.Timeout <= 0 {
		healthConfig.Config.L7APICheck.Timeout = 5 * time.Second
	}

	return &healthCheckerImpl{
		circuitBreaker: circuitBreaker,
		config:         healthConfig,
		healthStore:    healthStore,
		upstreamStore:  upstreamStore,
		stateCache:     NewStateCache(redisClient),
		httpClient: &http.Client{
			Timeout: healthConfig.Config.L7APICheck.Timeout,
		},
		stopCh:       make(chan struct{}),
		leaderNodeID: fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano()),
	}
}

func (h *healthCheckerImpl) Start(ctx context.Context) error {
	if !h.config.Config.Enabled {
		slog.InfoContext(ctx, "Health checker is disabled")
		return nil
	}

	slog.InfoContext(ctx, "Starting health checker", "leader_node", h.leaderNodeID)

	h.wg.Add(1)
	go h.runLeaderElection(ctx)

	if h.config.Config.L7APICheck.Enabled {
		h.wg.Add(1)
		go h.runL7APICheckRoutine(ctx)
	}
	if h.config.Config.InferenceCheck.Enabled {
		h.wg.Add(1)
		go h.runInferenceCheckRoutine(ctx)
	}
	return nil
}

func (h *healthCheckerImpl) Stop() error {
	close(h.stopCh)
	h.wg.Wait()
	return nil
}

func (h *healthCheckerImpl) runLeaderElection(ctx context.Context) {
	defer h.wg.Done()
	h.updateLeadership(ctx)
	ticker := time.NewTicker(healthLeaderHeartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.updateLeadership(ctx)
		}
	}
}

func (h *healthCheckerImpl) updateLeadership(ctx context.Context) {
	if !h.stateCache.Enabled() {
		h.isLeader.Store(true)
		return
	}

	if h.isLeader.Load() {
		renewed, err := h.stateCache.RenewLeader(ctx, healthLeaderElectionKey, h.leaderNodeID, healthLeaderTTL)
		if err == nil && renewed {
			return
		}
		h.isLeader.Store(false)
	}

	acquired, err := h.stateCache.TryAcquireLeader(ctx, healthLeaderElectionKey, h.leaderNodeID, healthLeaderTTL)
	if err != nil {
		slog.WarnContext(ctx, "failed to acquire health checker leadership", "error", err)
		h.isLeader.Store(false)
		return
	}
	h.isLeader.Store(acquired)

	var currentLeader string
	if acquired {
		currentLeader = h.leaderNodeID
	} else {
		leader, err := h.stateCache.GetLeader(ctx, healthLeaderElectionKey)
		if err == nil {
			currentLeader = leader
		} else {
			currentLeader = "unknown"
		}
	}

	if currentLeader != h.lastSeenLeader {
		slog.InfoContext(ctx, "health checker leadership changed",
			"node_id", h.leaderNodeID,
			"leader_node_id", currentLeader,
		)
		h.lastSeenLeader = currentLeader
	}
}

func (h *healthCheckerImpl) runL7APICheckRoutine(ctx context.Context) {
	defer h.wg.Done()

	interval := h.config.Config.L7APICheck.Interval
	if interval == 0 {
		interval = 60 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	h.performL7APIChecks(ctx)
	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.performL7APIChecks(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (h *healthCheckerImpl) runInferenceCheckRoutine(ctx context.Context) {
	defer h.wg.Done()

	interval := h.config.Config.InferenceCheck.Interval
	if interval == 0 {
		interval = 300 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	h.performInferenceChecks(ctx)
	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.performInferenceChecks(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (h *healthCheckerImpl) performL7APIChecks(ctx context.Context) {
	if !h.isLeader.Load() {
		return
	}

	upstreams, err := h.upstreamStore.ListHealthCheckEnabled(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get upstreams for health check", "error", err)
		return
	}

	var wg sync.WaitGroup
	for _, upstream := range upstreams {
		wg.Add(1)
		go func(u *database.Upstream) {
			defer wg.Done()
			result := h.performL7APICheck(ctx, u)
			h.updateHealthState(ctx, result)
		}(upstream)
	}
	wg.Wait()
}

func (h *healthCheckerImpl) performInferenceChecks(ctx context.Context) {
	if !h.isLeader.Load() {
		return
	}

	upstreams, err := h.upstreamStore.ListHealthCheckEnabled(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get upstreams for inference check", "error", err)
		return
	}

	var wg sync.WaitGroup
	for _, upstream := range upstreams {
		wg.Add(1)
		go func(u *database.Upstream) {
			defer wg.Done()
			result := h.performInferenceCheck(ctx, u)
			h.updateHealthState(ctx, result)
		}(upstream)
	}
	wg.Wait()
}

func (h *healthCheckerImpl) performL7APICheck(ctx context.Context, upstream *database.Upstream) *types.HealthCheckResult {
	startTime := time.Now()
	result := &types.HealthCheckResult{
		UpstreamID: upstream.ID,
		Provider:   upstream.Provider,
		ModelName:  upstream.ModelName,
		Endpoint:   upstream.URL,
		CheckType:  types.HealthCheckTypeL7API,
		Timestamp:  startTime,
	}

	if !strings.HasSuffix(upstream.URL, "/chat/completions") {
		result.Healthy = true
		return result
	}

	modelsURL := strings.TrimSuffix(upstream.URL, "/chat/completions") + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		slog.WarnContext(ctx, "Failed to create health check request", "upstream_id", upstream.ID, "url", modelsURL, "error", err)
		result.Error = err.Error()
		return result
	}

	if err = types.ApplyRequestAuthHeaders(req.Header, upstream.AuthHeader); err != nil {
		slog.WarnContext(ctx, "Failed to apply auth headers", "upstream_id", upstream.ID, "url", modelsURL, "error", err)
		result.Error = err.Error()
		return result
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		slog.WarnContext(ctx, "Failed to perform health check", "upstream_id", upstream.ID, "url", modelsURL, "error", err)
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.LatencyMs = time.Since(startTime).Milliseconds()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Healthy = true
	} else {
		result.Healthy = false
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		slog.WarnContext(ctx, "Health check probe failed with non-2xx status", "upstream_id", upstream.ID, "url", modelsURL, "status_code", resp.StatusCode)
	}
	return result
}

func (h *healthCheckerImpl) performInferenceCheck(ctx context.Context, upstream *database.Upstream) *types.HealthCheckResult {
	startTime := time.Now()
	result := &types.HealthCheckResult{
		UpstreamID: upstream.ID,
		Provider:   upstream.Provider,
		ModelName:  upstream.ModelName,
		Endpoint:   upstream.URL,
		CheckType:  types.HealthCheckTypeInference,
		Timestamp:  startTime,
	}

	reqBody := map[string]interface{}{
		"model": upstream.ModelName,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
		"max_tokens": 1,
		"stream":     false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		slog.WarnContext(ctx, "Failed to marshal inference check body", "upstream_id", upstream.ID, "error", err)
		result.Error = err.Error()
		return result
	}

	req, err := http.NewRequestWithContext(ctx, "POST", upstream.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		slog.WarnContext(ctx, "Failed to create inference check request", "upstream_id", upstream.ID, "url", upstream.URL, "error", err)
		result.Error = err.Error()
		return result
	}
	req.Header.Set("Content-Type", "application/json")

	if err = types.ApplyRequestAuthHeaders(req.Header, upstream.AuthHeader); err != nil {
		slog.WarnContext(ctx, "Failed to apply auth headers for inference check", "upstream_id", upstream.ID, "error", err)
		result.Error = err.Error()
		return result
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		slog.WarnContext(ctx, "Failed to perform inference check", "upstream_id", upstream.ID, "url", upstream.URL, "error", err)
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.LatencyMs = time.Since(startTime).Milliseconds()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Healthy = true
	} else {
		result.Healthy = false
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		slog.WarnContext(ctx, "Inference check failed with non-2xx status", "upstream_id", upstream.ID, "url", upstream.URL, "status_code", resp.StatusCode)
	}
	return result
}

// healthStateToGaugeValue maps HealthState to Prometheus gauge value.
// 0=unhealthy, 1=degraded, 2=healthy
func healthStateToGaugeValue(state types.HealthState) float64 {
	switch state {
	case types.HealthStateHealthy:
		return 2
	case types.HealthStateDegraded:
		return 1
	default:
		return 0
	}
}
func (h *healthCheckerImpl) updateHealthState(ctx context.Context, result *types.HealthCheckResult) {
	if result == nil {
		return
	}

	existingState, err := h.healthStore.GetByUpstreamID(ctx, result.UpstreamID)
	if err != nil {
		existingState = &database.AIGatewayUpstreamHealthState{
			UpstreamID:  result.UpstreamID,
			HealthState: string(types.HealthStateHealthy),
			LastCheckAt: result.Timestamp,
		}
	}

	threshold := h.config.Config.HealthRules.ConsecutiveFailuresForUnhealthy
	if threshold <= 0 {
		threshold = 3
	}
	latencyThreshold := h.config.Config.HealthRules.LatencyThresholdForDegraded

	if result.Healthy {
		existingState.ConsecutiveFailures = 0
		existingState.LastError = ""
		existingState.HealthState = string(types.HealthStateHealthy)
		if latencyThreshold > 0 && result.LatencyMs > latencyThreshold.Milliseconds() {
			existingState.HealthState = string(types.HealthStateDegraded)
		}
	} else {
		existingState.ConsecutiveFailures++
		existingState.LastError = result.Error
		if existingState.ConsecutiveFailures >= threshold {
			existingState.HealthState = string(types.HealthStateUnhealthy)
		}
	}

	existingState.LastCheckAt = result.Timestamp
	existingState.LatencyMs = result.LatencyMs

	if err := h.healthStore.Upsert(ctx, existingState); err != nil {
		slog.ErrorContext(ctx, "Failed to upsert health state",
			"error", err,
			"upstream_id", result.UpstreamID)
		return
	}

	status := &types.ProviderHealthStatus{
		UpstreamID:          existingState.UpstreamID,
		HealthState:         types.HealthState(existingState.HealthState),
		LastCheckAt:         existingState.LastCheckAt,
		LastError:           existingState.LastError,
		ConsecutiveFailures: existingState.ConsecutiveFailures,
		LatencyMs:           existingState.LatencyMs,
	}
	_ = h.stateCache.SetHealthState(ctx, status, healthStateCacheTTL)
	// Update Prometheus metrics
	if prom.AIGatewayUpstreamHealthState != nil {
		prom.AIGatewayUpstreamHealthState.WithLabelValues(
			strconv.FormatInt(result.UpstreamID, 10),
			result.ModelName,
			result.Provider,
			result.Endpoint,
			string(status.HealthState),
		).Set(healthStateToGaugeValue(status.HealthState))
	}

	// When inference check passes, close circuit breaker if it was open
	if result.Healthy && result.CheckType == types.HealthCheckTypeInference && h.circuitBreaker != nil {
		if err := h.circuitBreaker.ForceClose(ctx, result.UpstreamID); err != nil {
			slog.WarnContext(ctx, "Failed to close circuit breaker after successful inference check",
				"error", err,
				"upstream_id", result.UpstreamID)
		}
	}
}

func (h *healthCheckerImpl) CheckNow(ctx context.Context, upstreamID int64) (*types.HealthCheckResult, error) {
	upstream, err := h.upstreamStore.GetByID(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	result := h.performL7APICheck(ctx, upstream)
	h.updateHealthState(ctx, result)
	return result, nil
}

func (h *healthCheckerImpl) GetHealthState(ctx context.Context, upstreamID int64) (*types.ProviderHealthStatus, error) {
	if h.stateCache.Enabled() {
		cached, err := h.stateCache.GetHealthState(ctx, upstreamID)
		if err == nil {
			return cached, nil
		}
	}

	state, err := h.healthStore.GetByUpstreamID(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	status := &types.ProviderHealthStatus{
		UpstreamID:          state.UpstreamID,
		HealthState:         types.HealthState(state.HealthState),
		LastCheckAt:         state.LastCheckAt,
		LastError:           state.LastError,
		ConsecutiveFailures: state.ConsecutiveFailures,
		LatencyMs:           state.LatencyMs,
	}
	_ = h.stateCache.SetHealthState(ctx, status, healthStateCacheTTL)
	return status, nil
}

func (h *healthCheckerImpl) GetAllHealthStates(ctx context.Context) ([]types.ProviderHealthStatus, error) {
	healthy, err := h.healthStore.GetAllHealthy(ctx)
	if err != nil {
		return nil, err
	}
	unhealthy, err := h.healthStore.GetAllUnhealthy(ctx)
	if err != nil {
		return nil, err
	}
	results := make([]types.ProviderHealthStatus, 0, len(healthy)+len(unhealthy))
	for _, state := range healthy {
		results = append(results, types.ProviderHealthStatus{
			UpstreamID:          state.UpstreamID,
			HealthState:         types.HealthState(state.HealthState),
			LastCheckAt:         state.LastCheckAt,
			LastError:           state.LastError,
			ConsecutiveFailures: state.ConsecutiveFailures,
			LatencyMs:           state.LatencyMs,
		})
	}
	for _, state := range unhealthy {
		results = append(results, types.ProviderHealthStatus{
			UpstreamID:          state.UpstreamID,
			HealthState:         types.HealthState(state.HealthState),
			LastCheckAt:         state.LastCheckAt,
			LastError:           state.LastError,
			ConsecutiveFailures: state.ConsecutiveFailures,
			LatencyMs:           state.LatencyMs,
		})
	}
	return results, nil
}
