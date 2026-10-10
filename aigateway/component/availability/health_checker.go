package availability

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"opencsg.com/csghub-server/aigateway/sample"
	"opencsg.com/csghub-server/aigateway/types"
	prom "opencsg.com/csghub-server/builder/prometheus"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

const (
	healthStateCacheTTL                     = 30 * time.Second
	healthLeaderElectionKey                 = "health-checker"
	healthLeaderTTL                         = 15 * time.Second
	healthLeaderHeartbeat                   = 5 * time.Second
	defaultHealthCheckL7Interval            = 60 * time.Second
	defaultHealthCheckL7Timeout             = 15 * time.Second
	defaultMultimodalInferenceCheckInterval = 60 * time.Minute
	defaultHealthCheckProbeWorkers          = 32
	defaultHealthCheckPersistenceWorkers    = 4
)

var errStaleHealthCheckResult = errors.New("stale health check result")

type HealthChecker interface {
	Start(ctx context.Context) error
	Stop() error
	GetHealthState(ctx context.Context, upstreamID int64) (*types.ProviderHealthStatus, error)
}

type HealthCheckerConfig struct {
	Config             types.HealthCheckConfig
	ProbeWorkers       int
	PersistenceWorkers int
}

type healthCheckerImpl struct {
	circuitBreaker    CircuitBreaker
	config            HealthCheckerConfig
	healthStore       database.AIGatewayUpstreamHealthStateStore
	upstreamStore     database.UpstreamStore
	llmConfigStore    database.LLMConfigStore
	stateCache        StateCache
	httpClient        *http.Client
	sampleRegistry    *sample.Registry
	stopCh            chan struct{}
	wg                sync.WaitGroup
	stopOnce          sync.Once
	cancel            context.CancelFunc
	workerCtx         context.Context
	probeJobs         chan healthProbeJob
	persistenceShards []chan healthPersistenceJob
	sweepRunning      atomic.Bool
	leaderNodeID      string
	isLeader          atomic.Bool
	lastSeenLeader    string
	multimodalProbes  multimodalProbeScheduler
	now               func() time.Time
}

type healthProbeJob struct {
	ctx        context.Context
	upstream   *database.Upstream
	completion chan<- struct{}
}

type healthPersistenceJob struct {
	ctx      context.Context
	result   *types.HealthCheckResult
	policy   types.SampleExecutionPolicy
	response chan<- healthPersistenceResult
}

type healthPersistenceResult struct {
	health    multimodalHealthState
	persisted bool
}

func NewHealthChecker(
	circuitBreaker CircuitBreaker,
	cfg *config.Config,
	healthStore database.AIGatewayUpstreamHealthStateStore,
	upstreamStore database.UpstreamStore,
	llmConfigStore database.LLMConfigStore,
	redisClient cache.RedisClient,
) HealthChecker {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "aigateway"
	}

	healthConfig := HealthCheckerConfig{
		ProbeWorkers:       cfg.AIGateway.HealthCheckProbeWorkers,
		PersistenceWorkers: cfg.AIGateway.HealthCheckPersistenceWorkers,
		Config: types.HealthCheckConfig{
			Enabled: cfg.AIGateway.HealthCheckEnabled,
			L7APICheck: types.L7APICheckConfig{
				Enabled:  cfg.AIGateway.HealthCheckL7APIEnabled,
				Interval: time.Duration(cfg.AIGateway.HealthCheckL7APIInterval) * time.Second,
				Timeout:  time.Duration(cfg.AIGateway.HealthCheckL7APITimeout) * time.Second,
			},
			MultimodalInferenceInterval: time.Duration(cfg.AIGateway.HealthCheckModalInferenceInterval) * time.Second,
			HealthRules: types.HealthRulesConfig{
				ConsecutiveFailuresForUnhealthy:       cfg.AIGateway.HealthCheckConsecutiveFailures,
				LatencyThresholdForDegraded:           time.Duration(cfg.AIGateway.HealthCheckLatencyDegradedMs) * time.Millisecond,
				MultimodalLatencyThresholdForDegraded: time.Duration(cfg.AIGateway.HealthCheckMultimodalLatencyDegradedMs) * time.Millisecond,
			},
		},
	}

	if healthConfig.Config.HealthRules.ConsecutiveFailuresForUnhealthy <= 0 {
		healthConfig.Config.HealthRules.ConsecutiveFailuresForUnhealthy = 3
	}
	if healthConfig.Config.L7APICheck.Interval <= 0 {
		healthConfig.Config.L7APICheck.Interval = defaultHealthCheckL7Interval
	}
	if healthConfig.Config.L7APICheck.Timeout <= 0 {
		healthConfig.Config.L7APICheck.Timeout = defaultHealthCheckL7Timeout
	}
	if healthConfig.Config.MultimodalInferenceInterval <= 0 {
		healthConfig.Config.MultimodalInferenceInterval = defaultMultimodalInferenceCheckInterval
	}

	stateCache := NewStateCache(redisClient)
	return &healthCheckerImpl{
		circuitBreaker:   circuitBreaker,
		config:           healthConfig,
		healthStore:      healthStore,
		upstreamStore:    upstreamStore,
		llmConfigStore:   llmConfigStore,
		stateCache:       stateCache,
		httpClient:       &http.Client{},
		sampleRegistry:   sample.NewDefaultRegistry(),
		stopCh:           make(chan struct{}),
		leaderNodeID:     fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano()),
		multimodalProbes: newMultimodalProbeScheduler(stateCache),
		now:              time.Now,
	}
}

func (h *healthCheckerImpl) Start(ctx context.Context) error {
	if !h.config.Config.Enabled {
		slog.InfoContext(ctx, "Health checker is disabled")
		return nil
	}

	probeWorkers := h.probeWorkerCount()
	persistenceWorkers := h.persistenceWorkerCount()
	h.workerCtx, h.cancel = context.WithCancel(ctx)
	h.probeJobs = make(chan healthProbeJob, probeWorkers)
	// One queue per persistence worker, sharded by upstream ID: results for the
	// same upstream always land on the same FIFO queue, so persistence for one
	// upstream executes in enqueue order even across consecutive sweeps.
	shardCap := max(1, 2*probeWorkers/persistenceWorkers)
	h.persistenceShards = make([]chan healthPersistenceJob, persistenceWorkers)

	for range probeWorkers {
		h.wg.Add(1)
		go h.runProbeWorker(h.workerCtx)
	}
	for i := range persistenceWorkers {
		h.persistenceShards[i] = make(chan healthPersistenceJob, shardCap)
		h.wg.Add(1)
		go h.runPersistenceWorker(h.workerCtx, h.persistenceShards[i])
	}

	slog.InfoContext(ctx, "Starting health checker",
		"leader_node", h.leaderNodeID,
		"probe_workers", probeWorkers,
		"persistence_workers", persistenceWorkers,
		"persistence_queue_capacity", shardCap*persistenceWorkers,
	)

	h.wg.Add(1)
	go h.runLeaderElection(h.workerCtx)

	if h.config.Config.L7APICheck.Enabled {
		h.wg.Add(1)
		go h.runL7APICheckRoutine(h.workerCtx)
	}
	return nil
}

func (h *healthCheckerImpl) Stop() error {
	h.stopOnce.Do(func() {
		close(h.stopCh)
		if h.cancel != nil {
			h.cancel()
		}
	})
	h.wg.Wait()
	return nil
}

func (h *healthCheckerImpl) probeWorkerCount() int {
	if h.config.ProbeWorkers > 0 {
		return h.config.ProbeWorkers
	}
	return defaultHealthCheckProbeWorkers
}

func (h *healthCheckerImpl) persistenceWorkerCount() int {
	if h.config.PersistenceWorkers > 0 {
		return h.config.PersistenceWorkers
	}
	return defaultHealthCheckPersistenceWorkers
}

func (h *healthCheckerImpl) runProbeWorker(ctx context.Context) {
	defer h.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-h.probeJobs:
			if prom.AIGatewayHealthCheckProbeActive != nil {
				prom.AIGatewayHealthCheckProbeActive.Inc()
			}
			jobCtx, cancelJob := workerJobContext(ctx, job.ctx)
			h.performUpstreamHealthCheck(jobCtx, job.upstream)
			cancelJob()
			if prom.AIGatewayHealthCheckProbeActive != nil {
				prom.AIGatewayHealthCheckProbeActive.Dec()
			}
			select {
			case job.completion <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (h *healthCheckerImpl) runPersistenceWorker(ctx context.Context, jobs chan healthPersistenceJob) {
	defer h.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-jobs:
			if prom.AIGatewayHealthCheckPersistenceQueue != nil {
				prom.AIGatewayHealthCheckPersistenceQueue.Set(float64(h.persistenceQueueDepth()))
			}
			if prom.AIGatewayHealthCheckPersistenceActive != nil {
				prom.AIGatewayHealthCheckPersistenceActive.Inc()
			}
			jobCtx, cancelJob := workerJobContext(ctx, job.ctx)
			health, persisted := h.updateHealthStateForPolicy(jobCtx, job.result, job.policy)
			cancelJob()
			if prom.AIGatewayHealthCheckPersistenceActive != nil {
				prom.AIGatewayHealthCheckPersistenceActive.Dec()
			}
			if job.response == nil {
				continue
			}
			select {
			case job.response <- healthPersistenceResult{health: health, persisted: persisted}:
			case <-ctx.Done():
				return
			}
		}
	}
}

func workerJobContext(workerCtx, jobCtx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(jobCtx)
	stopWorkerCancellation := context.AfterFunc(workerCtx, cancel)
	return ctx, func() {
		stopWorkerCancellation()
		cancel()
	}
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
		h.setLeadership(true)
		return
	}

	if h.isLeader.Load() {
		renewed, err := h.stateCache.RenewLeader(ctx, healthLeaderElectionKey, h.leaderNodeID, healthLeaderTTL)
		if err == nil && renewed {
			return
		}
		h.setLeadership(false)
	}

	acquired, err := h.stateCache.TryAcquireLeader(ctx, healthLeaderElectionKey, h.leaderNodeID, healthLeaderTTL)
	if err != nil {
		slog.WarnContext(ctx, "failed to acquire health checker leadership", "error", err)
		h.setLeadership(false)
		return
	}
	h.setLeadership(acquired)

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

func (h *healthCheckerImpl) setLeadership(isLeader bool) {
	if h.isLeader.Swap(isLeader) != isLeader {
		h.multimodalProbes.reset()
	}
}

func (h *healthCheckerImpl) runL7APICheckRoutine(ctx context.Context) {
	defer h.wg.Done()

	interval := h.l7Interval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

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

func (h *healthCheckerImpl) performL7APIChecks(ctx context.Context) {
	if !h.isLeader.Load() {
		return
	}
	if !h.sweepRunning.CompareAndSwap(false, true) {
		if prom.AIGatewayHealthCheckSweepsSkipped != nil {
			prom.AIGatewayHealthCheckSweepsSkipped.Inc()
		}
		slog.WarnContext(ctx, "health check sweep skipped because another sweep is running")
		return
	}
	defer h.sweepRunning.Store(false)

	upstreams, err := h.upstreamStore.ListHealthCheckEnabled(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get upstreams for health check", "error", err)
		return
	}
	activeUpstreamIDs := make([]int64, 0, len(upstreams))
	for _, upstream := range upstreams {
		activeUpstreamIDs = append(activeUpstreamIDs, upstream.ID)
	}
	h.multimodalProbes.cleanup(activeUpstreamIDs)

	startedAt := time.Now()
	defer func() {
		if prom.AIGatewayHealthCheckSweepDuration != nil {
			prom.AIGatewayHealthCheckSweepDuration.Observe(time.Since(startedAt).Seconds())
		}
	}()
	completion := make(chan struct{}, len(upstreams))
	submitted := 0
	for _, upstream := range upstreams {
		if h.probeJobs == nil {
			h.performUpstreamHealthCheck(ctx, upstream)
			continue
		}
		select {
		case h.probeJobs <- healthProbeJob{ctx: ctx, upstream: upstream, completion: completion}:
			submitted++
		case <-ctx.Done():
			return
		case <-h.stopCh:
			return
		}
	}
	for range submitted {
		select {
		case <-completion:
		case <-ctx.Done():
			return
		case <-h.stopCh:
			return
		}
	}
	duration := time.Since(startedAt)
	slog.InfoContext(ctx, "health check sweep completed",
		"upstream_count", len(upstreams),
		"duration", duration,
		"persistence_queue_depth", h.persistenceQueueDepth(),
	)
	if duration > h.l7Interval() {
		slog.WarnContext(ctx, "health check sweep exceeded interval; overlapping sweep skipped",
			"duration", duration,
			"interval", h.l7Interval(),
		)
	}
}

func (h *healthCheckerImpl) performUpstreamHealthCheck(ctx context.Context, upstream *database.Upstream) {
	provider, ok := h.getSampleRegistry().Find(upstream.URL)
	if !ok {
		slog.InfoContext(ctx, "Skipping health check for unsupported upstream protocol",
			"upstream_id", upstream.ID,
			"model_name", upstream.ModelName,
			"url", upstream.URL)
		return
	}
	inferencePolicy, err := provider.ExecutionPolicy(types.SampleKindInference)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get inference sample policy",
			"error", err,
			"upstream_id", upstream.ID,
			"url", upstream.URL)
		return
	}
	if inferencePolicy.Timeout <= 0 {
		slog.ErrorContext(ctx, "Invalid inference sample timeout",
			"timeout", inferencePolicy.Timeout,
			"upstream_id", upstream.ID,
			"url", upstream.URL)
		return
	}

	now := h.currentTime()
	plan := multimodalProbePlan{inferenceDue: true}
	probePolicy := h.multimodalProbePolicy()
	probeTTL := h.multimodalProbeStateTTL(inferencePolicy.Timeout)
	if inferencePolicy.Multimodal {
		state, err := h.loadMultimodalProbeState(ctx, upstream.ID, probePolicy, probeTTL)
		if err != nil {
			slog.WarnContext(ctx, "Failed to load multimodal probe state",
				"error", err,
				"upstream_id", upstream.ID)
			return
		}
		plan = state.plan(now)
		if plan.skipAll {
			return
		}
	} else if err := h.multimodalProbes.deleteIfCached(ctx, upstream.ID); err != nil {
		slog.WarnContext(ctx, "Failed to delete stale multimodal probe state",
			"error", err,
			"upstream_id", upstream.ID)
	}

	l7Outcome := h.performL7APICheckOutcome(ctx, upstream)
	result := l7Outcome.result
	if l7Outcome.inferenceFallbackRequired {
		if inferencePolicy.Multimodal {
			if !plan.inferenceDue {
				if err := h.multimodalProbes.recordInferenceOnly(ctx, upstream.ID, probeTTL); err != nil {
					slog.WarnContext(ctx, "Failed to record inference-only multimodal probe mode",
						"error", err,
						"upstream_id", upstream.ID)
				}
				return
			}
			reserved, err := h.reserveMultimodalInference(ctx, upstream.ID, inferencePolicy.Timeout, probeTTL)
			if err != nil {
				slog.WarnContext(ctx, "Failed to reserve multimodal inference probe",
					"error", err,
					"upstream_id", upstream.ID)
				return
			}
			if !reserved {
				return
			}
		}
		result = h.performInferenceCheck(ctx, upstream, inferencePolicy.Timeout)
		result.LatencyMs += l7Outcome.result.LatencyMs
		result.UsedInferenceFallback = true
		if !inferencePolicy.Multimodal {
			h.persistHealthStateAsync(ctx, result, inferencePolicy)
			return
		}
		health, persisted := h.persistHealthStateForPolicy(ctx, result, inferencePolicy)
		if !persisted {
			return
		}
		if err := h.multimodalProbes.recordInferenceSchedule(
			ctx, upstream.ID, h.currentTime(), true, health.Inference.ConsecutiveFailures, probePolicy, probeTTL,
		); err != nil {
			slog.WarnContext(ctx, "Failed to persist multimodal inference schedule",
				"error", err,
				"upstream_id", upstream.ID)
		}
		return
	}
	if !result.Healthy {
		h.persistHealthStateAsync(ctx, result, inferencePolicy)
		return
	}
	if inferencePolicy.Multimodal {
		_, persisted := h.persistHealthStateForPolicy(ctx, result, inferencePolicy)
		if !persisted {
			return
		}
		err := h.multimodalProbes.recordL7Available(ctx, upstream.ID, probeTTL)
		if err != nil {
			slog.WarnContext(ctx, "Failed to record available multimodal L7 endpoint",
				"error", err,
				"upstream_id", upstream.ID)
		}
		if err != nil {
			return
		}
		if !plan.inferenceDue {
			return
		}
		reserved, err := h.reserveMultimodalInference(ctx, upstream.ID, inferencePolicy.Timeout, probeTTL)
		if err != nil {
			slog.WarnContext(ctx, "Failed to reserve multimodal inference probe",
				"error", err,
				"upstream_id", upstream.ID)
			return
		}
		if !reserved {
			return
		}
	}

	result = h.performInferenceCheck(ctx, upstream, inferencePolicy.Timeout)
	if !inferencePolicy.Multimodal {
		h.persistHealthStateAsync(ctx, result, inferencePolicy)
		return
	}
	health, persisted := h.persistHealthStateForPolicy(ctx, result, inferencePolicy)
	if !persisted {
		return
	}
	if err := h.multimodalProbes.recordInferenceSchedule(
		ctx, upstream.ID, h.currentTime(), false, health.Inference.ConsecutiveFailures, probePolicy, probeTTL,
	); err != nil {
		slog.WarnContext(ctx, "Failed to persist multimodal inference schedule",
			"error", err,
			"upstream_id", upstream.ID)
	}
}

func (h *healthCheckerImpl) currentTime() time.Time {
	if h.now == nil {
		return time.Now()
	}
	return h.now()
}

func (h *healthCheckerImpl) l7Interval() time.Duration {
	if interval := h.config.Config.L7APICheck.Interval; interval > 0 {
		return interval
	}
	return defaultHealthCheckL7Interval
}

func (h *healthCheckerImpl) multimodalInferenceInterval() time.Duration {
	if interval := h.config.Config.MultimodalInferenceInterval; interval > 0 {
		return interval
	}
	return defaultMultimodalInferenceCheckInterval
}

func (h *healthCheckerImpl) consecutiveFailureThreshold() int {
	if threshold := h.config.Config.HealthRules.ConsecutiveFailuresForUnhealthy; threshold > 0 {
		return threshold
	}
	return 3
}

func (h *healthCheckerImpl) multimodalProbePolicy() multimodalProbePolicy {
	return multimodalProbePolicy{
		retryInterval:     h.l7Interval(),
		inferenceInterval: h.multimodalInferenceInterval(),
		failureThreshold:  h.consecutiveFailureThreshold(),
	}
}

func (h *healthCheckerImpl) multimodalHealthPolicy() multimodalHealthPolicy {
	return multimodalHealthPolicy{
		failureThreshold:          h.consecutiveFailureThreshold(),
		l7LatencyThreshold:        h.config.Config.HealthRules.LatencyThresholdForDegraded,
		inferenceLatencyThreshold: h.config.Config.HealthRules.MultimodalLatencyThresholdForDegraded,
	}
}

func (h *healthCheckerImpl) multimodalProbeStateTTL(inferenceTimeout time.Duration) time.Duration {
	reservationWindow := inferenceTimeout + h.l7Interval()
	base := h.multimodalInferenceInterval()
	if reservationWindow > base {
		base = reservationWindow
	}
	return 2 * base
}

func (h *healthCheckerImpl) loadMultimodalProbeState(
	ctx context.Context,
	upstreamID int64,
	policy multimodalProbePolicy,
	ttl time.Duration,
) (multimodalProbeState, error) {
	state, found, err := h.multimodalProbes.load(ctx, upstreamID)
	if err != nil || found {
		return state, err
	}

	if h.multimodalProbes.stateCache == nil || !h.multimodalProbes.stateCache.Enabled() {
		if err := h.multimodalProbes.store(ctx, upstreamID, state, ttl); err != nil {
			return multimodalProbeState{}, err
		}
		return state, nil
	}

	if h.healthStore != nil {
		healthState, healthErr := h.healthStore.GetByUpstreamID(ctx, upstreamID)
		switch {
		case healthErr == nil:
			state, healthErr = bootstrapMultimodalProbeState(healthState, policy)
			if healthErr != nil {
				return multimodalProbeState{}, healthErr
			}
		case errors.Is(healthErr, sql.ErrNoRows):
			// A new upstream has no persisted cadence and is immediately due.
		default:
			return multimodalProbeState{}, fmt.Errorf("load health state for multimodal probe bootstrap: %w", healthErr)
		}
	}

	if err := h.multimodalProbes.store(ctx, upstreamID, state, ttl); err != nil {
		h.multimodalProbes.setLocal(upstreamID, state)
		return multimodalProbeState{}, err
	}
	return state, nil
}

func bootstrapMultimodalProbeState(
	healthState *database.AIGatewayUpstreamHealthState,
	policy multimodalProbePolicy,
) (multimodalProbeState, error) {
	if healthState == nil {
		return multimodalProbeState{}, nil
	}
	metadata, found, err := readMultimodalHealthState(healthState.Metadata)
	if err != nil {
		return multimodalProbeState{}, err
	}
	if !found {
		// Legacy rows do not contain inference cadence facts and are immediately due.
		return multimodalProbeState{}, nil
	}
	state := multimodalProbeState{}
	if !metadata.Inference.LastCheckedAt.IsZero() {
		state.nextInferenceAt = nextMultimodalInferenceAt(
			metadata.Inference.LastCheckedAt,
			metadata.Inference.ConsecutiveFailures,
			policy,
		)
	}
	return state, nil
}

func (h *healthCheckerImpl) reserveMultimodalInference(
	ctx context.Context,
	upstreamID int64,
	inferenceTimeout time.Duration,
	ttl time.Duration,
) (bool, error) {
	now := h.currentTime()
	return h.multimodalProbes.reserveInference(
		ctx,
		upstreamID,
		now,
		now.Add(inferenceTimeout+h.l7Interval()),
		ttl,
	)
}

func (h *healthCheckerImpl) getSampleRegistry() *sample.Registry {
	if h.sampleRegistry == nil {
		return sample.NewDefaultRegistry()
	}
	return h.sampleRegistry
}

type sampleCheckOutcome struct {
	result                    *types.HealthCheckResult
	inferenceFallbackRequired bool
}

func (h *healthCheckerImpl) performL7APICheck(ctx context.Context, upstream *database.Upstream) *types.HealthCheckResult {
	return h.performL7APICheckOutcome(ctx, upstream).result
}

func (h *healthCheckerImpl) performL7APICheckOutcome(ctx context.Context, upstream *database.Upstream) *sampleCheckOutcome {
	return h.performSampleCheck(ctx, upstream, types.SampleKindL7API, h.config.Config.L7APICheck.Timeout)
}

func (h *healthCheckerImpl) performInferenceCheck(ctx context.Context, upstream *database.Upstream, timeout time.Duration) *types.HealthCheckResult {
	return h.performSampleCheck(ctx, upstream, types.SampleKindInference, timeout).result
}

func (h *healthCheckerImpl) performSampleCheck(ctx context.Context, upstream *database.Upstream, kind types.SampleKind, timeout time.Duration) *sampleCheckOutcome {
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// PostgreSQL stores timestamptz with microsecond precision. Normalize the
	// event clock so an exact retry remains equal after a database round trip.
	startTime := time.Now().UTC().Truncate(time.Microsecond)
	checkType := types.HealthCheckTypeInference
	if kind == types.SampleKindL7API {
		checkType = types.HealthCheckTypeL7API
	}
	result := &types.HealthCheckResult{
		UpstreamID: upstream.ID,
		Provider:   upstream.Provider,
		ModelName:  upstream.ModelName,
		Endpoint:   upstream.URL,
		CheckType:  checkType,
		Timestamp:  startTime,
	}
	outcome := &sampleCheckOutcome{result: result}

	provider, ok := h.getSampleRegistry().Find(upstream.URL)
	if !ok {
		// performL7APICheck is also used directly by existing callers and tests.
		// The scheduled health-check routine skips unsupported protocols before
		// reaching this point, leaving their persisted state unchanged.
		result.Healthy = true
		return outcome
	}

	headers := make(http.Header)
	if err := types.ApplyRequestAuthHeaders(headers, upstream.AuthHeader); err != nil {
		result.Error = err.Error()
		return outcome
	}
	var tasks []string
	if kind == types.SampleKindInference {
		tasks = database.ResolveLLMTasks(ctx, h.llmConfigStore, upstream.LLMConfigID)
	}
	execution, err := provider.Execute(checkCtx, kind, types.SampleInput{
		Endpoint:             upstream.URL,
		Headers:              headers,
		Model:                upstream.ModelName,
		Text:                 "hi",
		Tasks:                tasks,
		MaxResponseBodyBytes: 1024,
	}, h.httpClient)
	if err != nil {
		result.Error = err.Error()
		return outcome
	}
	if execution == nil || execution.Request == nil {
		result.Error = "sample execution returned no request"
		return outcome
	}

	log := slog.With(
		slog.Int64("upstream_id", upstream.ID),
		slog.String("model_name", upstream.ModelName),
		slog.String("url", execution.Request.Endpoint),
	)
	result.LatencyMs = execution.Latency.Milliseconds()
	if execution.InferenceFallbackRequired {
		outcome.inferenceFallbackRequired = true
		return outcome
	}
	if execution.Error != nil {
		log.WarnContext(ctx, "Failed to perform health check", "error", execution.Error)
		result.Error = execution.Error.Error()
		return outcome
	}
	if execution.StatusCode >= 200 && execution.StatusCode < 300 {
		result.Healthy = true
	} else {
		result.Healthy = false
		result.Error = fmt.Sprintf("HTTP %d, url: %s, response: %s", execution.StatusCode, execution.Request.Endpoint, string(execution.ResponseBody))
		log.WarnContext(ctx, "Health check sample failed with non-2xx status",
			"status_code", execution.StatusCode,
			"url", execution.Request.Endpoint,
			"response", string(execution.ResponseBody))
	}
	return outcome
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

func (h *healthCheckerImpl) updateHealthState(ctx context.Context, result *types.HealthCheckResult) bool {
	if result == nil {
		return false
	}
	if result.Multimodal {
		_, persisted := h.updateMultimodalHealthState(ctx, result)
		return persisted
	}

	threshold := h.config.Config.HealthRules.ConsecutiveFailuresForUnhealthy
	if threshold <= 0 {
		threshold = 3
	}
	latencyThreshold := h.config.Config.HealthRules.LatencyThresholdForDegraded
	oldHealthState := ""
	var persistedLastCheckedAt time.Time
	existingState, err := h.healthStore.MutateByUpstreamID(ctx, database.AIGatewayUpstreamHealthStateMutation{
		UpstreamID:      result.UpstreamID,
		CreateIfMissing: true,
		Mutate: func(state *database.AIGatewayUpstreamHealthState) error {
			if state == nil {
				return errors.New("mutate health state: nil state")
			}
			oldHealthState = state.HealthState
			if !state.LastCheckAt.IsZero() && !result.Timestamp.After(state.LastCheckAt) {
				persistedLastCheckedAt = state.LastCheckAt
				return errStaleHealthCheckResult
			}
			if result.Healthy {
				state.ConsecutiveFailures = 0
				state.LastError = ""
				state.HealthState = string(types.HealthStateHealthy)
				if latencyThreshold > 0 && result.LatencyMs > latencyThreshold.Milliseconds() {
					state.HealthState = string(types.HealthStateDegraded)
				}
			} else {
				state.ConsecutiveFailures++
				state.LastError = result.Error
				if state.ConsecutiveFailures >= threshold {
					state.HealthState = string(types.HealthStateUnhealthy)
				}
			}
			state.LastCheckAt = result.Timestamp
			state.LatencyMs = result.LatencyMs
			return nil
		},
	})
	if err != nil {
		if errors.Is(err, errStaleHealthCheckResult) {
			logStaleHealthCheckResult(ctx, result, persistedLastCheckedAt)
			return false
		}
		slog.ErrorContext(ctx, "Failed to mutate health state",
			"error", err,
			"upstream_id", result.UpstreamID)
		return false
	}
	if existingState == nil {
		slog.ErrorContext(ctx, "Mutate health state returned nil without error",
			"upstream_id", result.UpstreamID)
		return false
	}

	if !result.Healthy {
		slog.InfoContext(ctx, "health check unhealthy",
			"upstream_id", result.UpstreamID,
			"model_name", result.ModelName,
			"provider", result.Provider,
			"consecutive_failures", existingState.ConsecutiveFailures,
			"threshold", threshold,
			"error", result.Error,
			"check_type", result.CheckType,
			"latency_ms", result.LatencyMs,
		)
	}
	if existingState.HealthState != oldHealthState {
		slog.InfoContext(ctx, "health state changed",
			"upstream_id", result.UpstreamID,
			"model_name", result.ModelName,
			"provider", result.Provider,
			"old_state", oldHealthState,
			"new_state", existingState.HealthState,
			"consecutive_failures", existingState.ConsecutiveFailures,
			"last_error", existingState.LastError,
			"latency_ms", result.LatencyMs,
			"latency_threshold_ms", latencyThreshold.Milliseconds(),
			"multimodal", result.Multimodal,
			"check_type", result.CheckType,
		)
	}

	h.publishHealthState(ctx, result, existingState)

	// When inference check passes, close circuit breaker if it was open
	if result.Healthy && result.CheckType == types.HealthCheckTypeInference && h.circuitBreaker != nil {
		if err := h.circuitBreaker.ForceClose(ctx, result.UpstreamID); err != nil {
			slog.WarnContext(ctx, "Failed to close circuit breaker after successful inference check",
				"error", err,
				"upstream_id", result.UpstreamID)
		}
	}
	return true
}

func (h *healthCheckerImpl) updateMultimodalHealthState(
	ctx context.Context,
	result *types.HealthCheckResult,
) (multimodalHealthState, bool) {
	if result == nil {
		return multimodalHealthState{}, false
	}
	oldHealthState := ""
	var health multimodalHealthState
	var projection multimodalHealthProjection
	var persistedLastCheckedAt time.Time
	existingState, err := h.healthStore.MutateByUpstreamID(ctx, database.AIGatewayUpstreamHealthStateMutation{
		UpstreamID:      result.UpstreamID,
		CreateIfMissing: true,
		Mutate: func(state *database.AIGatewayUpstreamHealthState) error {
			if state == nil {
				return errors.New("mutate multimodal health state: nil state")
			}
			oldHealthState = state.HealthState
			current, _, err := readMultimodalHealthState(state.Metadata)
			if err != nil {
				return err
			}
			persistedLastCheckedAt = multimodalLastCheckedAt(current, result.CheckType)
			var applied bool
			health, applied = reduceMultimodalHealth(current, multimodalProbeEvent{
				Kind:      result.CheckType,
				Healthy:   result.Healthy,
				CheckedAt: result.Timestamp,
				LatencyMs: result.LatencyMs,
				Error:     result.Error,
			})
			if !applied {
				return errStaleHealthCheckResult
			}
			projection = projectMultimodalHealth(health, h.multimodalHealthPolicy())
			writeMultimodalHealthState(state, health)
			state.HealthState = string(projection.healthState)
			state.LastCheckAt = projection.lastCheckAt
			state.LastError = projection.lastError
			state.ConsecutiveFailures = projection.consecutiveFailures
			state.LatencyMs = projection.latencyMs
			return nil
		},
	})
	if err != nil {
		if errors.Is(err, errStaleHealthCheckResult) {
			logStaleHealthCheckResult(ctx, result, persistedLastCheckedAt)
			return multimodalHealthState{}, false
		}
		logLevel := slog.LevelError
		if errors.Is(err, errInvalidMultimodalHealthState) {
			logLevel = slog.LevelWarn
		}
		slog.Log(ctx, logLevel, "Failed to mutate multimodal health state",
			"error", err,
			"upstream_id", result.UpstreamID)
		return multimodalHealthState{}, false
	}
	if existingState == nil {
		slog.ErrorContext(ctx, "Mutate multimodal health state returned nil without error",
			"upstream_id", result.UpstreamID)
		return multimodalHealthState{}, false
	}
	h.publishHealthState(ctx, result, existingState)

	if !result.Healthy {
		slog.InfoContext(ctx, "health check unhealthy",
			"upstream_id", result.UpstreamID,
			"model_name", result.ModelName,
			"provider", result.Provider,
			"consecutive_failures", projection.consecutiveFailures,
			"threshold", h.consecutiveFailureThreshold(),
			"error", result.Error,
			"check_type", result.CheckType,
			"latency_ms", result.LatencyMs,
		)
	}
	if existingState.HealthState != oldHealthState {
		slog.InfoContext(ctx, "health state changed",
			"upstream_id", result.UpstreamID,
			"model_name", result.ModelName,
			"provider", result.Provider,
			"old_state", oldHealthState,
			"new_state", existingState.HealthState,
			"consecutive_failures", existingState.ConsecutiveFailures,
			"last_error", existingState.LastError,
			"latency_ms", result.LatencyMs,
			"multimodal", true,
			"check_type", result.CheckType,
		)
	}

	if result.Healthy && result.CheckType == types.HealthCheckTypeInference && h.circuitBreaker != nil {
		if err := h.circuitBreaker.ForceClose(ctx, result.UpstreamID); err != nil {
			slog.WarnContext(ctx, "Failed to close circuit breaker after successful inference check",
				"error", err,
				"upstream_id", result.UpstreamID)
		}
	}
	return health, true
}

func logStaleHealthCheckResult(
	ctx context.Context,
	result *types.HealthCheckResult,
	persistedLastCheckedAt time.Time,
) {
	slog.DebugContext(ctx, "Ignoring stale health check result",
		"upstream_id", result.UpstreamID,
		"check_type", result.CheckType,
		"checked_at", result.Timestamp,
		"persisted_last_checked_at", persistedLastCheckedAt,
	)
}

func (h *healthCheckerImpl) publishHealthState(
	ctx context.Context,
	result *types.HealthCheckResult,
	existingState *database.AIGatewayUpstreamHealthState,
) {
	if existingState == nil {
		slog.WarnContext(ctx, "Skip publishing nil health state",
			"upstream_id", func() int64 {
				if result != nil {
					return result.UpstreamID
				}
				return 0
			}())
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
	if prom.AIGatewayUpstreamHealthState != nil && result != nil {
		prom.AIGatewayUpstreamHealthState.WithLabelValues(
			strconv.FormatInt(result.UpstreamID, 10),
			result.ModelName,
			result.Provider,
			string(status.HealthState),
		).Set(healthStateToGaugeValue(status.HealthState))
	}
}

func (h *healthCheckerImpl) updateHealthStateForPolicy(
	ctx context.Context,
	result *types.HealthCheckResult,
	policy types.SampleExecutionPolicy,
) (multimodalHealthState, bool) {
	if result != nil {
		result.Multimodal = policy.Multimodal
	}
	if policy.Multimodal {
		return h.updateMultimodalHealthState(ctx, result)
	}
	return multimodalHealthState{}, h.updateHealthState(ctx, result)
}

func (h *healthCheckerImpl) persistHealthStateForPolicy(
	ctx context.Context,
	result *types.HealthCheckResult,
	policy types.SampleExecutionPolicy,
) (multimodalHealthState, bool) {
	if result == nil {
		return multimodalHealthState{}, false
	}
	if len(h.persistenceShards) == 0 {
		return h.updateHealthStateForPolicy(ctx, result, policy)
	}

	response := make(chan healthPersistenceResult, 1)
	job := healthPersistenceJob{ctx: ctx, result: result, policy: policy, response: response}
	select {
	case h.persistenceShard(result.UpstreamID) <- job:
		h.recordPersistenceQueueDepth()
	case <-ctx.Done():
		return multimodalHealthState{}, false
	case <-h.workerCtx.Done():
		return multimodalHealthState{}, false
	}

	select {
	case persisted := <-response:
		return persisted.health, persisted.persisted
	case <-ctx.Done():
		return multimodalHealthState{}, false
	case <-h.workerCtx.Done():
		return multimodalHealthState{}, false
	}
}

// persistHealthStateAsync enqueues persistence without waiting for the outcome.
// The probe's job context is canceled as soon as the probe returns, so the
// queued job must run on the worker context instead of the caller's context.
func (h *healthCheckerImpl) persistHealthStateAsync(
	ctx context.Context,
	result *types.HealthCheckResult,
	policy types.SampleExecutionPolicy,
) {
	if result == nil {
		return
	}
	if len(h.persistenceShards) == 0 {
		h.updateHealthStateForPolicy(ctx, result, policy)
		return
	}

	job := healthPersistenceJob{ctx: h.workerCtx, result: result, policy: policy}
	select {
	case h.persistenceShard(result.UpstreamID) <- job:
		h.recordPersistenceQueueDepth()
	case <-ctx.Done():
	case <-h.workerCtx.Done():
	}
}

// persistenceShard returns the FIFO persistence queue for an upstream, keeping
// per-upstream persistence in enqueue order across concurrent sweeps.
func (h *healthCheckerImpl) persistenceShard(upstreamID int64) chan healthPersistenceJob {
	return h.persistenceShards[upstreamID%int64(len(h.persistenceShards))]
}

func (h *healthCheckerImpl) persistenceQueueDepth() int {
	depth := 0
	for _, shard := range h.persistenceShards {
		depth += len(shard)
	}
	return depth
}

func (h *healthCheckerImpl) recordPersistenceQueueDepth() {
	if prom.AIGatewayHealthCheckPersistenceQueue != nil {
		prom.AIGatewayHealthCheckPersistenceQueue.Set(float64(h.persistenceQueueDepth()))
	}
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
