package admission

import (
	"context"
	"fmt"
	"hash/crc32"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	prom "opencsg.com/csghub-server/builder/prometheus"
	"opencsg.com/csghub-server/builder/store/cache"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestMain(m *testing.M) {
	// The e2e test verifies Prometheus gauge values against Redis state, so
	// the metric variables must be registered for this test binary.
	prom.InitMetrics()
	m.Run()
}

// The e2e simulation models sustained load against three models, each backed
// by three upstreams with different capacities:
//
//   - session-affinity requests pin one upstream (KV-cache style routing):
//     never re-selected, rejected when full,
//   - round-robin requests may be re-selected by capacity-aware admission,
//   - requests complete with real usage, fail without usage, or "crash"
//     (never finalized — the expired-lease cleanup must reclaim them),
//   - long streamers cross window boundaries and renew their lease,
//   - some requests trigger an availability-style failover mid-flight
//     (finalize old lease + pinned acquire on another upstream).
//
// Time is simulated through miniredis (the scripts read Redis TIME), so a
// multi-window run completes in milliseconds of wall clock. At every
// checkpoint the Prometheus gauges are compared against the raw Redis state,
// and the accounting ledger (per upstream per window) is compared against the
// Redis RPM/TPM counters to prove conservation: no over-admission, no leaked
// reservations, no double counting.
func TestCapacityAdmissionEndToEnd(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	clk := &simClock{now: time.Unix(1_700_000_000, 0)}
	mr.SetTime(clk.now)
	redisClient := cache.NewCacheWithClient(ctx, redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctrl := NewCapacityAdmissionController(redisClient, CapacityAdmissionOptions{
		LeaseTTL:                60 * time.Second,
		FailOpen:                true,
		PromptCharsPerToken:     4,
		CompletionTokenEstimate: 1000,
		MaxTokenEstimate:        32768,
		RetryAfterHint:          5 * time.Second,
		RenewInterval:           0, // renewals are driven by the simulation
	})

	models := buildE2EModels()
	allUpstreamIDs := map[string][]int64{}
	for _, m := range models {
		ids := make([]int64, 0, len(m.Upstreams))
		for _, up := range m.Upstreams {
			ids = append(ids, up.ID)
		}
		allUpstreamIDs[m.ID] = ids
	}
	maxConc := map[int64]int{}
	for _, m := range models {
		for _, up := range m.Upstreams {
			maxConc[up.ID] = up.CapacityPolicy.MaxConcurrency
		}
	}

	sim := newE2ESimulation(t, ctrl, rdb, models, rngForE2E())

	const (
		tick        = time.Second
		totalTicks  = 150 // 2.5 windows — crosses at least one rollover
		verifyEvery = 10  // periodic metrics/Redis correspondence check
	)

	for tickIdx := 0; tickIdx < totalTicks; tickIdx++ {
		sim.arrivals(ctx, clk.now)
		sim.fallbacks(ctx, clk.now)
		sim.completions(ctx, clk.now)
		sim.renewals(ctx, clk.now)
		if tickIdx%verifyEvery == verifyEvery-1 {
			sim.verifyCheckpoint(ctx, clk.now, allUpstreamIDs, maxConc)
		}
		clk.now = clk.now.Add(tick)
		mr.SetTime(clk.now)
	}

	// Drain: advance past every service time and lease TTL so completions
	// run and crashed leases become reclaimable by the cleanup.
	for i := 0; i < 200; i++ {
		sim.completions(ctx, clk.now)
		clk.now = clk.now.Add(tick)
		mr.SetTime(clk.now)
	}

	// --- Final invariants ---

	// 1. Cleanup reclaims every slot: no lease survives the drain.
	for _, model := range models {
		states, err := ctrl.Observe(ctx, model.ID, allUpstreamIDs[model.ID])
		require.NoError(t, err)
		for _, st := range states {
			require.Equal(t, int64(0), st.CurrentConcurrency,
				"no lease may survive after drain (model %s upstream %d)", model.ID, st.UpstreamID)
			empty, err := rdb.HLen(ctx, capacityLeaseKey(model.ID, st.UpstreamID)).Result()
			require.NoError(t, err)
			require.Equal(t, int64(0), empty, "lease metadata must be empty after drain")
		}
	}

	// 2. Metrics correspondence after an Observe-driven refresh: gauges must
	// equal the raw Redis state exactly.
	for _, model := range models {
		states, err := ctrl.Observe(ctx, model.ID, allUpstreamIDs[model.ID])
		require.NoError(t, err)
		ReportCapacityStates(model.ID, states)
	}
	verifyMetricsAgainstRedis(t, rdb, models, allUpstreamIDs)

	// 3. Lifecycle bookkeeping: every acquired lease was either explicitly
	// released (usage/failed/failover/abort) or crashed and reclaimed.
	require.Equal(t, sim.leasesAdmitted, sim.leasesReleased+sim.leasesCrashed,
		"leases admitted = released + crashed (no leaks)")
	require.Positive(t, sim.leasesAdmitted)
	require.Positive(t, sim.rejected, "sustained load must produce rejections")
	// The seeded load profile must actually exercise the hard paths —
	// otherwise the invariants above would hold vacuously.
	require.Positive(t, sim.leasesCrashed, "crash profile must occur")
	require.Positive(t, sim.failovers, "failover profile must occur")

	// 4. Session affinity: capacity admission never moved a session request
	// off its pinned upstream (availability-style failovers are tracked
	// separately and allowed to move).
	for sessionModel, upstreamID := range sim.sessionPinned {
		serving := sim.sessionServed[sessionModel]
		require.NotEmpty(t, serving, "pinned session %s was never admitted", sessionModel)
		for _, served := range serving {
			require.Equal(t, upstreamID, served,
				"session %s drifted from its pinned upstream", sessionModel)
		}
	}

	// 5. Rejections never carried a lease.
	require.Equal(t, 0, sim.rejectsWithLease)
}

// --- simulation building blocks ---

type simClock struct{ now time.Time }

type simRequest struct {
	lease          *types.AdmissionLease
	acquiredAtMS   int64     // current lease acquire time (RPM entry score)
	expiryAt       time.Time // lease TTL deadline (crash reclamation point)
	finishAt       time.Time // service completion (crash profile: past the TTL)
	fallbackAt     time.Time // availability failover trigger; zero = none
	nextRenewAt    time.Time
	usage          *token.Usage
	modelID        string
	routeMode      string // "session" | "roundrobin"
	released       bool   // the live reservation has been reclaimed from the sum
	failed         bool
	crashed        bool
	doubleFinalize bool // exercises the idempotent second finalize
}

type e2eSimulation struct {
	t      *testing.T
	ctrl   CapacityAdmissionController
	rdb    *redis.Client
	models []*types.Model
	rng    *rand.Rand
	seq    int
	rr     int

	inflight []*simRequest

	sessionPinned map[string]int64 // "session|model" -> pinned upstream ID
	sessionServed map[string][]int64

	leasesAdmitted   int // every successful lease acquisition
	leasesReleased   int // every explicit Finalize that took ownership
	leasesCrashed    int // crash-profile requests, reclaimed by cleanup
	failovers        int // availability-style mid-flight failovers
	rejected         int
	rejectsWithLease int

	// Sliding-window ledgers: every successful acquisition and every
	// committed usage contribution is recorded with its timestamp; the
	// checkpoint filters by the 60s window cutoff and compares against
	// Redis (post-Observe, so all due GC/reclamation has happened).
	acquires []acquireRec
	usages   []usageRec
	all      []*simRequest // every admitted request, completed included
}

type acquireRec struct {
	upstreamID int64
	atMS       int64
}

type usageRec struct {
	upstreamID int64
	tokens     int64
	atMS       int64
}

func newE2ESimulation(t *testing.T, ctrl CapacityAdmissionController, rdb *redis.Client, models []*types.Model, rng *rand.Rand) *e2eSimulation {
	return &e2eSimulation{
		t:             t,
		ctrl:          ctrl,
		rdb:           rdb,
		models:        models,
		rng:           rng,
		sessionPinned: map[string]int64{},
		sessionServed: map[string][]int64{},
	}
}

func rngForE2E() *rand.Rand { return rand.New(rand.NewSource(42)) }

// buildE2EModels creates 3 models × 3 upstreams; capacities differ per
// upstream and per model.
func buildE2EModels() []*types.Model {
	models := make([]*types.Model, 0, 3)
	for m := 0; m < 3; m++ {
		modelID := fmt.Sprintf("model-%s", string(rune('a'+m)))
		upstreams := make([]commontypes.UpstreamConfig, 0, 3)
		for u := 0; u < 3; u++ {
			scale := int64(m+1) * int64(u+1)
			upstreams = append(upstreams, commontypes.UpstreamConfig{
				ID:      int64(m*10 + u + 1),
				URL:     fmt.Sprintf("https://upstream%d.example.com/v1", m*10+u+1),
				Enabled: true,
				CapacityPolicy: &commontypes.CapacityPolicy{
					Enabled:        true,
					MaxConcurrency: 4 * int(scale),
					MaxRPM:         30 * int(scale),
					MaxTPM:         30000 * scale,
				},
			})
		}
		models = append(models, &types.Model{
			BaseModel: types.BaseModel{ID: modelID, OwnedBy: "e2e"},
			Upstreams: upstreams,
		})
	}
	return models
}

// pinUpstream mimics the session-hash router: a session key deterministically
// maps to one candidate index.
func pinUpstream(sessionKey string, numUpstreams int) int {
	return int(crc32.ChecksumIEEE([]byte(sessionKey)) % uint32(numUpstreams))
}

func (s *e2eSimulation) arrivals(ctx context.Context, now time.Time) {
	num := 2 + s.rng.Intn(4) // 2..5 requests per simulated second
	for i := 0; i < num; i++ {
		s.seq++
		model := s.models[s.rng.Intn(len(s.models))]
		promptText := makeE2EString(100 + s.rng.Intn(2000))
		// Multimodal profile (~15%): no TPM reservation — the text-based
		// estimate is fiction for image/audio requests; RPM and concurrency
		// still gate them, and actual usage is committed at finalize.
		var est int64
		if s.rng.Float64() < 0.15 {
			est = types.AdmissionNoTPMEstimate
		} else {
			est = s.ctrl.EstimateAdmissionTokens(promptText)
		}
		routeMode := "roundrobin"
		if s.seq%2 == 0 {
			routeMode = "session"
		}

		var decision *types.AdmissionDecision
		sessionKey := fmt.Sprintf("session-%d", s.seq%25)
		pinKey := sessionKey + "|" + model.ID
		if routeMode == "session" {
			pinned := model.Upstreams[pinUpstream(sessionKey, len(model.Upstreams))]
			decision = s.ctrl.Acquire(ctx, model, pinned.ID, est)
		} else {
			preferred := model.Upstreams[s.rr%len(model.Upstreams)]
			s.rr++
			decision = s.ctrl.Check(ctx, types.CapacityAdmissionRequest{
				Model:               model,
				PreferredUpstreamID: preferred.ID,
				AllowSelect:         true,
				EstimatedTokens:     est,
			})
		}

		if decision == nil {
			continue // admission does not apply
		}
		if decision.Action != types.AdmissionAdmit {
			s.rejected++
			if decision.Lease != nil {
				s.rejectsWithLease++
			}
			continue
		}

		// Post-acquire state can never exceed the configured limits: the
		// atomic acquire script cannot over-admit.
		for _, st := range decision.States {
			if st.Selected {
				require.LessOrEqual(s.t, st.CurrentConcurrency, int64(st.MaxConcurrency))
				require.LessOrEqual(s.t, st.CurrentRPM, int64(st.MaxRPM))
				require.LessOrEqual(s.t, st.CurrentTPM, st.MaxTPM)
			}
		}

		s.leasesAdmitted++
		s.acquires = append(s.acquires, acquireRec{decision.Lease.UpstreamID, now.UnixMilli()})
		if routeMode == "session" {
			// Record the pin only on successful admission: the invariant is
			// "sessions admitted by capacity admission never drift off their
			// pinned upstream" (Acquire must never re-select).
			s.sessionPinned[pinKey] = decision.Lease.UpstreamID
			s.sessionServed[pinKey] = append(s.sessionServed[pinKey], decision.SelectedUpstreamID)
		}

		req := &simRequest{
			lease:        decision.Lease,
			acquiredAtMS: now.UnixMilli(),
			expiryAt:     now.Add(60 * time.Second), // lease TTL
			modelID:      model.ID,
			routeMode:    routeMode,
		}
		s.scheduleLifecycle(req, now)
		s.inflight = append(s.inflight, req)
		s.all = append(s.all, req)
	}
}

// scheduleLifecycle assigns the service profile of one admitted request:
// crash / failed attempt / long stream / normal, plus the optional
// availability failover and the double-finalize drill. Actual usage is
// derived from the lease's reservation (heavy fixed usage for the no-TPM
// multimodal profile).
func (s *e2eSimulation) scheduleLifecycle(req *simRequest, now time.Time) {
	roll := s.rng.Float64()
	switch {
	case roll < 0.05:
		// Crash: the pod dies mid-request; no finalize ever runs and the
		// finishAt is past the lease TTL so the cleanup must reclaim it.
		req.crashed = true
		req.finishAt = now.Add(90 * time.Second)
		s.leasesCrashed++
		return
	case roll < 0.12:
		// Reached the upstream but produced no usable usage.
		req.failed = true
	case roll < 0.22:
		// Long stream crossing at least one window boundary.
		req.finishAt = now.Add(time.Duration(65+s.rng.Intn(40)) * time.Second)
	default:
		req.finishAt = now.Add(time.Duration(1+s.rng.Intn(15)) * time.Second)
	}
	if roll >= 0.05 && roll < 0.9 {
		req.fallbackAt = now.Add(time.Duration(2+s.rng.Intn(8)) * time.Second)
	}
	req.nextRenewAt = now.Add(20 * time.Second)
	req.doubleFinalize = s.seq%97 == 0

	var actual int64
	if req.lease.ReservedTokens <= 0 {
		// Multimodal profile: image/audio requests consume heavy actual
		// usage despite reserving nothing; the finalize commit still lands
		// it in the TPM window.
		actual = 500 + s.rng.Int63n(2500)
	} else {
		actual = int64(float64(req.lease.ReservedTokens) * (0.3 + s.rng.Float64()*1.1))
		if actual < 1 {
			actual = 1
		}
	}
	req.usage = &token.Usage{TotalTokens: actual}
}

// fallbacks triggers the availability-style failover for requests whose
// fallback time has come: finalize the old lease without usage, then pinned
// acquire on another upstream (no re-selection).
func (s *e2eSimulation) fallbacks(ctx context.Context, now time.Time) {
	remaining := s.inflight[:0]
	for _, req := range s.inflight {
		if req.crashed || req.failed || req.fallbackAt.IsZero() || now.Before(req.fallbackAt) || now.After(req.finishAt) {
			remaining = append(remaining, req)
			continue
		}
		s.leasesReleased++ // the old lease is finalized without usage
		s.failovers++
		s.ctrl.Finalize(ctx, req.lease, nil)

		model := s.modelByID(req.modelID)
		var target *commontypes.UpstreamConfig
		for i := range model.Upstreams {
			if model.Upstreams[i].ID != req.lease.UpstreamID {
				target = &model.Upstreams[i]
				break
			}
		}
		decision := s.ctrl.Acquire(ctx, model, target.ID, req.lease.ReservedTokens)
		if decision == nil || decision.Action != types.AdmissionAdmit {
			// Fallback rejected: the request aborts with 429 and its
			// lifecycle ends here (the old lease was already released).
			req.crashed = true
			req.released = true
			req.finishAt = now // dropped from in-flight immediately
			continue
		}
		// Serve on the new upstream.
		s.leasesAdmitted++
		s.acquires = append(s.acquires, acquireRec{decision.Lease.UpstreamID, now.UnixMilli()})
		req.lease = decision.Lease
		req.acquiredAtMS = now.UnixMilli()
		req.released = false
		req.expiryAt = now.Add(60 * time.Second)
		req.finishAt = now.Add(time.Duration(1+s.rng.Intn(10)) * time.Second)
		req.fallbackAt = time.Time{}
		req.nextRenewAt = now.Add(20 * time.Second)
		remaining = append(remaining, req)
	}
	s.inflight = remaining
}

func (s *e2eSimulation) completions(ctx context.Context, now time.Time) {
	remaining := s.inflight[:0]
	for _, req := range s.inflight {
		if now.Before(req.finishAt) {
			remaining = append(remaining, req)
			continue
		}
		if req.crashed {
			// Never finalized: dropped from tracking; the expired-lease
			// cleanup reclaims its slot and reservation (verified later).
			continue
		}
		if req.failed {
			s.ctrl.Finalize(ctx, req.lease, nil)
		} else {
			s.ctrl.Finalize(ctx, req.lease, req.usage)
			s.usages = append(s.usages, usageRec{req.lease.UpstreamID, req.usage.TotalTokens, now.UnixMilli()})
		}
		req.released = true
		s.leasesReleased++
		if req.doubleFinalize {
			// The Orchestrator safety net finalizes again without usage —
			// must be a Redis no-op (no ledger change, no over-release).
			s.ctrl.Finalize(ctx, req.lease, nil)
		}
	}
	s.inflight = remaining
}

func (s *e2eSimulation) renewals(ctx context.Context, now time.Time) {
	remaining := s.inflight[:0]
	for _, req := range s.inflight {
		if req.crashed || req.failed || req.fallbackAt.IsZero() || now.Before(req.nextRenewAt) || !now.Before(req.finishAt) {
			remaining = append(remaining, req)
			continue
		}
		require.True(s.t, s.ctrl.Renew(ctx, req.lease),
			"an in-flight, non-crashed lease must always renew successfully")
		req.nextRenewAt = now.Add(20 * time.Second)
		remaining = append(remaining, req)
	}
	s.inflight = remaining
}

func (s *e2eSimulation) modelByID(id string) *types.Model {
	for _, m := range s.models {
		if m.ID == id {
			return m
		}
	}
	return nil
}

// verifyCheckpoint runs the periodic "定时看" step: an Observe-driven
// collector refresh (the future background job), then verifies that
//   - the observed state matches the raw Redis counters,
//   - the Prometheus gauges match the observed state,
//   - the sliding-window ledgers match Redis exactly: RPM counts every
//     acquisition of the last 60s, the TPM running sum equals live
//     reservations plus un-aged usage contributions,
//   - no upstream ever exceeded its concurrency limit.
//
// The comparison runs AFTER the checkpoint's Observe, so every GC and crash
// reclamation that is due has deterministically happened.
func (s *e2eSimulation) verifyCheckpoint(ctx context.Context, now time.Time, allUpstreamIDs map[string][]int64, maxConc map[int64]int) {
	cutoffMS := now.UnixMilli() - 60000

	for _, model := range s.models {
		ids := allUpstreamIDs[model.ID]
		states, err := s.ctrl.Observe(ctx, model.ID, ids)
		require.NoError(s.t, err)
		require.Len(s.t, states, len(ids))

		for _, st := range states {
			conc, rpm, tpm := rawRedisCapacity(s.t, s.rdb, model.ID, st.UpstreamID)
			require.Equal(s.t, conc, st.CurrentConcurrency, "observe conc must match raw Redis")
			require.Equal(s.t, rpm, st.CurrentRPM, "observe rpm must match raw Redis")
			require.Equal(s.t, tpm, st.CurrentTPM, "observe tpm must match raw Redis")
			require.Equal(s.t, s.expectedRPM(st.UpstreamID, cutoffMS), rpm,
				"sliding-window rpm ledger mismatch for upstream %d", st.UpstreamID)
			require.Equal(s.t, s.expectedTPM(st.UpstreamID, now), tpm,
				"sliding-window tpm ledger mismatch for upstream %d", st.UpstreamID)
			require.LessOrEqual(s.t, st.CurrentConcurrency, int64(maxConc[st.UpstreamID]),
				"over-admission detected on upstream %d", st.UpstreamID)
		}
		ReportCapacityStates(model.ID, states)
	}

	verifyMetricsAgainstRedis(s.t, s.rdb, s.models, allUpstreamIDs)
}

// expectedRPM counts the acquisitions inside the sliding window.
func (s *e2eSimulation) expectedRPM(upstreamID int64, cutoffMS int64) int64 {
	n := 0
	for _, a := range s.acquires {
		if a.upstreamID == upstreamID && a.atMS >= cutoffMS {
			n++
		}
	}
	return int64(n)
}

// expectedTPM computes the running sum from the ledgers: reservations of
// every request whose live lease has not been finalized (crashed requests
// whose lease TTL already passed are reclaimed by the checkpoint's Observe
// cleanup) plus usage contributions that have not aged out.
func (s *e2eSimulation) expectedTPM(upstreamID int64, now time.Time) int64 {
	nowMS := now.UnixMilli()
	cutoffMS := nowMS - 60000
	var total int64
	for _, r := range s.all {
		if r.lease == nil || r.lease.UpstreamID != upstreamID || r.released {
			continue
		}
		if r.crashed && r.expiryAt.UnixMilli() <= nowMS {
			continue // reclaimed by the checkpoint's Observe cleanup
		}
		total += r.lease.ReservedTokens
	}
	for _, u := range s.usages {
		if u.upstreamID == upstreamID && u.atMS >= cutoffMS {
			total += u.tokens
		}
	}
	return total
}

// verifyMetricsAgainstRedis compares the Prometheus capacity gauges with the
// raw Redis counters for every model/upstream/dimension.
func verifyMetricsAgainstRedis(t *testing.T, rdb *redis.Client, models []*types.Model, allUpstreamIDs map[string][]int64) {
	t.Helper()
	gauges := gatherCapacityGauges(t)
	for _, model := range models {
		for _, upstreamID := range allUpstreamIDs[model.ID] {
			conc, rpm, tpm := rawRedisCapacity(t, rdb, model.ID, upstreamID)
			expected := map[string]int64{"concurrency": conc, "rpm": rpm, "tpm": tpm}
			for dim, want := range expected {
				key := model.ID + "|" + strconv.FormatInt(upstreamID, 10) + "|" + dim
				got, ok := gauges[key]
				require.True(t, ok, "missing gauge %s", key)
				require.Equal(t, float64(want), got, "gauge %s diverged from Redis", key)
			}
		}
	}
}

// rawRedisCapacity reads the counters directly from Redis, bypassing the
// controller.
func rawRedisCapacity(t *testing.T, rdb *redis.Client, modelID string, upstreamID int64) (conc, rpm, tpm int64) {
	t.Helper()
	conc = rdb.ZCard(context.Background(), capacityConcurrencyKey(modelID, upstreamID)).Val()
	rpm = rdb.ZCard(context.Background(), capacityRPMKey(modelID, upstreamID)).Val()
	tpm = redisGetInt(t, rdb, capacityTPMSumKey(modelID, upstreamID))
	return conc, rpm, tpm
}

func redisGetInt(t *testing.T, rdb *redis.Client, key string) int64 {
	t.Helper()
	val, err := rdb.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return 0
	}
	require.NoError(t, err)
	n, err := strconv.ParseInt(val, 10, 64)
	require.NoError(t, err)
	return n
}

// gatherCapacityGauges collects csghub_aigateway_upstream_capacity_current
// into a "model|upstream|dimension" -> value map.
func gatherCapacityGauges(t *testing.T) map[string]float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	out := map[string]float64{}
	for _, mf := range families {
		if mf.GetName() != "csghub_aigateway_upstream_capacity_current" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			labels := map[string]string{}
			for _, lp := range metric.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			key := labels["model"] + "|" + labels["upstream_id"] + "|" + labels["dimension"]
			out[key] = metric.GetGauge().GetValue()
		}
	}
	return out
}

func makeE2EString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + i%26)
	}
	return string(b)
}
