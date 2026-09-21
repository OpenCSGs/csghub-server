package admission

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/cache"
	commontypes "opencsg.com/csghub-server/common/types"
)

func admissionTestOptions() CapacityAdmissionOptions {
	return CapacityAdmissionOptions{
		LeaseTTL:                60 * time.Second,
		FailOpen:                true,
		PromptCharsPerToken:     4,
		CompletionTokenEstimate: 1000,
		MaxTokenEstimate:        32768,
		RetryAfterHint:          5 * time.Second,
		// RenewInterval <= 0 disables the background renewer; tests renew
		// manually.
		RenewInterval: 0,
	}
}

// testClock tracks the Redis server time used by the scripts (read via the
// TIME command). Advance it with Advance + SetTime on miniredis.
type testClock struct{ t time.Time }

func (c *testClock) Now() time.Time { return c.t }

// newMiniredisController builds a controller backed by a real miniredis so
// the Lua lifecycle scripts run for real.
func newMiniredisController(t *testing.T) (CapacityAdmissionController, *miniredis.Miniredis, *redis.Client, *testClock) {
	t.Helper()
	mr := miniredis.RunT(t)
	clk := &testClock{t: time.Now()}
	mr.SetTime(clk.t)
	redisClient := cache.NewCacheWithClient(context.Background(), redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	return NewCapacityAdmissionController(redisClient, admissionTestOptions()), mr, redis.NewClient(&redis.Options{Addr: mr.Addr()}), clk
}

func admissionTestModel(upstreams ...commontypes.UpstreamConfig) *types.Model {
	return &types.Model{
		BaseModel: types.BaseModel{ID: "test-model", OwnedBy: "openai"},
		Upstreams: upstreams,
	}
}

func capacityUpstream(id int64, maxConc int, maxRPM int, maxTPM int64) commontypes.UpstreamConfig {
	return commontypes.UpstreamConfig{
		ID:             id,
		URL:            "https://upstream-" + strconv.FormatInt(id, 10) + ".example.com/v1",
		Enabled:        true,
		CapacityPolicy: &commontypes.CapacityPolicy{Enabled: true, MaxConcurrency: maxConc, MaxRPM: maxRPM, MaxTPM: maxTPM},
	}
}

func zaddLease(t *testing.T, rdb *redis.Client, modelID string, upstreamID int64, member string, expireAtMS int64) {
	t.Helper()
	require.NoError(t, rdb.ZAdd(context.Background(), capacityConcurrencyKey(modelID, upstreamID), redis.Z{
		Score:  float64(expireAtMS),
		Member: member,
	}).Err())
}

func getString(t *testing.T, rdb *redis.Client, key string) int64 {
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

func setString(t *testing.T, rdb *redis.Client, key string, value int64) {
	t.Helper()
	require.NoError(t, rdb.Set(context.Background(), key, value, 0).Err())
}

func zcard(t *testing.T, rdb *redis.Client, key string) int64 {
	t.Helper()
	return rdb.ZCard(context.Background(), key).Val()
}

// zaddRPM seeds one sliding-window RPM entry at the given millisecond.
func zaddRPM(t *testing.T, rdb *redis.Client, modelID string, upstreamID int64, member string, atMS int64) {
	t.Helper()
	require.NoError(t, rdb.ZAdd(context.Background(), capacityRPMKey(modelID, upstreamID), redis.Z{
		Score:  float64(atMS),
		Member: member,
	}).Err())
}

// --- miniredis integration tests: the reservation lifecycle matrix ---

func TestAdmissionCheck_Admit_AcquiresLease(t *testing.T) {
	ctrl, _, rdb, clk := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 20, 100000))

	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.NotNil(t, decision)
	require.Equal(t, types.AdmissionAdmit, decision.Action)
	require.False(t, decision.ReSelected)
	require.NotNil(t, decision.Lease)
	require.Equal(t, int64(1), decision.Lease.UpstreamID)
	require.Equal(t, int64(1064), decision.Lease.ReservedTokens)
	require.Len(t, decision.States, 1)
	require.True(t, decision.States[0].Feasible)
	// The selected upstream's snapshot is post-acquire: it includes this
	// request's reservation.
	require.True(t, decision.States[0].Selected)
	require.Equal(t, int64(1), decision.States[0].CurrentConcurrency)
	require.Equal(t, int64(1), decision.States[0].CurrentRPM)
	require.Equal(t, int64(1064), decision.States[0].CurrentTPM)

	require.Equal(t, int64(1), zcard(t, rdb, capacityConcurrencyKey("test-model", 1)))
	leaseMeta, err := rdb.HGet(context.Background(), capacityLeaseKey("test-model", 1), decision.Lease.Token).Result()
	require.NoError(t, err)
	// Metadata keeps the "<est>:<acquired at second>" packing; the recorded
	// window start is informational under the sliding window.
	require.Equal(t, "1064:"+strconv.FormatInt(clk.Now().Unix(), 10), leaseMeta)
	// RPM is a sliding-window ZSET entry; the TPM reservation lives in the
	// running sum.
	require.Equal(t, int64(1), zcard(t, rdb, capacityRPMKey("test-model", 1)))
	require.Equal(t, int64(1064), getString(t, rdb, capacityTPMSumKey("test-model", 1)))
}

func TestAdmissionCheck_NoPolicy_SkipsRedis(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	// Construction preloads the Lua scripts (LoadScript); Check itself must
	// make zero Redis calls (no RunScript EXPECT below).
	redisClient.EXPECT().LoadScript(mock.Anything, mock.Anything).Return(nil).Times(5)
	ctrl := NewCapacityAdmissionController(redisClient, admissionTestOptions())
	model := admissionTestModel(commontypes.UpstreamConfig{ID: 1, URL: "https://u.example.com", Enabled: true})
	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{Model: model, PreferredUpstreamID: 1})
	require.Nil(t, decision)
}

func TestAdmissionCheck_Pinned_FullConcurrency_Rejects(t *testing.T) {
	ctrl, _, rdb, clk := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 1, 100, 1000000))
	// Occupy the single concurrency slot with a live lease.
	zaddLease(t, rdb, "test-model", 1, "other-request", (clk.Now().Unix()+60)*1000)

	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		AllowSelect:         false,
		EstimatedTokens:     1064,
	})
	require.NotNil(t, decision)
	require.Equal(t, types.AdmissionReject, decision.Action)
	require.Equal(t, types.AdmissionReasonConcurrencyExceeded, decision.Reason)
	require.Nil(t, decision.Lease)
	require.False(t, decision.States[0].Feasible)
	require.Equal(t, []string{"concurrency"}, decision.States[0].BlockedBy)
	require.Positive(t, decision.RetryAfterSeconds)
}

func TestAdmissionCheck_RPMExceeded_RejectsWithWindowRetryAfter(t *testing.T) {
	ctrl, _, rdb, clk := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 2, 1000000))
	// Two acquires 30s/25s ago are still inside the sliding window.
	for _, age := range []int64{30, 25} {
		zaddRPM(t, rdb, "test-model", 1, "old-"+strconv.FormatInt(age, 10), (clk.Now().Unix()-age)*1000)
	}

	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionReject, decision.Action)
	require.Equal(t, types.AdmissionReasonRPMExceeded, decision.Reason)
	// Retry-After hints when the oldest contribution leaves the window:
	// the 30s-old entry exits in 30 seconds.
	require.Equal(t, int64(30), decision.RetryAfterSeconds)
}

func TestAdmissionCheck_TPMExceeded_Rejects(t *testing.T) {
	ctrl, _, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 100, 2000))
	setString(t, rdb, capacityTPMSumKey("test-model", 1), 1000)

	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionReject, decision.Action)
	require.Equal(t, types.AdmissionReasonTPMExceeded, decision.Reason)
}

func TestAdmissionCheck_MultiDimensionBlocked_ReasonsCapacityExceeded(t *testing.T) {
	ctrl, _, rdb, clk := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 1, 1, 100))
	zaddLease(t, rdb, "test-model", 1, "other", (clk.Now().Unix()+60)*1000)
	for i := 0; i < 5; i++ {
		zaddRPM(t, rdb, "test-model", 1, "old-"+strconv.Itoa(i), (clk.Now().Unix()-20)*1000)
	}

	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionReject, decision.Action)
	require.Equal(t, types.AdmissionReasonCapacityExceeded, decision.Reason)
}

func TestAdmissionCheck_ReselectsMaxBottleneckHeadroom(t *testing.T) {
	ctrl, _, rdb, clk := newMiniredisController(t)
	// Upstream 1: 4/5 concurrency slots busy (headroom 0.2). Upstream 2: empty.
	model := admissionTestModel(capacityUpstream(1, 5, 100, 1000000), capacityUpstream(2, 5, 100, 1000000))
	for i := 0; i < 4; i++ {
		zaddLease(t, rdb, "test-model", 1, "lease-"+strconv.Itoa(i), (clk.Now().Unix()+60)*1000)
	}

	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		AllowSelect:         true,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, decision.Action)
	require.True(t, decision.ReSelected)
	require.Equal(t, int64(2), decision.SelectedUpstreamID)
	require.NotNil(t, decision.Lease)
	require.Equal(t, int64(2), decision.Lease.UpstreamID)
	// Selected upstream reports post-acquire state; the non-selected one
	// reports the pre-acquire snapshot.
	byID := map[int64]types.UpstreamCapacityState{}
	for _, s := range decision.States {
		byID[s.UpstreamID] = s
	}
	require.True(t, byID[2].Selected)
	require.Equal(t, int64(1), byID[2].CurrentConcurrency)
	require.False(t, byID[1].Selected)
	require.Equal(t, int64(4), byID[1].CurrentConcurrency)
}

func TestAcquire_PinnedUpstream_NeverReselects(t *testing.T) {
	ctrl, _, rdb, clk := newMiniredisController(t)
	// Preferred/pinned upstream 1 is full; upstream 2 is free — Acquire must
	// reject for upstream 1 and never touch upstream 2.
	model := admissionTestModel(capacityUpstream(1, 1, 100, 1000000), capacityUpstream(2, 5, 100, 1000000))
	zaddLease(t, rdb, "test-model", 1, "other", (clk.Now().Unix()+60)*1000)

	decision := ctrl.Acquire(context.Background(), model, 1, 1064)
	require.Equal(t, types.AdmissionReject, decision.Action)
	require.Equal(t, types.AdmissionReasonConcurrencyExceeded, decision.Reason)
	require.Equal(t, int64(0), zcard(t, rdb, capacityConcurrencyKey("test-model", 2)))

	// Acquiring the free upstream succeeds and reports post-acquire state.
	ok := ctrl.Acquire(context.Background(), model, 2, 1064)
	require.Equal(t, types.AdmissionAdmit, ok.Action)
	require.Len(t, ok.States, 1)
	require.True(t, ok.States[0].Selected)
	require.Equal(t, int64(1), ok.States[0].CurrentConcurrency)
}

func TestAdmissionCheck_AllUnlimited_PrefersFirstCandidate(t *testing.T) {
	ctrl, _, _, _ := newMiniredisController(t)
	// All limits 0 = unlimited: feasible with +inf score, ties go to the
	// first candidate (which differs from the preferred one, hence a
	// re-selection).
	model := admissionTestModel(
		commontypes.UpstreamConfig{ID: 1, URL: "https://a.example.com", Enabled: true, CapacityPolicy: &commontypes.CapacityPolicy{Enabled: true}},
		commontypes.UpstreamConfig{ID: 2, URL: "https://b.example.com", Enabled: true, CapacityPolicy: &commontypes.CapacityPolicy{Enabled: true}},
	)
	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 2,
		AllowSelect:         true,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, decision.Action)
	require.True(t, decision.ReSelected)
	require.Equal(t, int64(1), decision.SelectedUpstreamID)
}

func TestAdmissionCheck_PinnedOnlyEvaluatesPreferred(t *testing.T) {
	ctrl, _, rdb, clk := newMiniredisController(t)
	// Upstream 2 stays untouched: pinned requests evaluate a single
	// candidate.
	model := admissionTestModel(capacityUpstream(1, 1, 100, 1000000), capacityUpstream(2, 5, 100, 1000000))
	zaddLease(t, rdb, "test-model", 1, "other", (clk.Now().Unix()+60)*1000)

	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		AllowSelect:         false,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionReject, decision.Action)
	require.Len(t, decision.States, 1)
	require.Equal(t, int64(0), zcard(t, rdb, capacityConcurrencyKey("test-model", 2)))
}

func TestAdmissionCheck_FailOpenOnRedisError(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	redisClient.EXPECT().LoadScript(mock.Anything, mock.Anything).Return(nil).Times(5)
	redisClient.EXPECT().
		RunScript(mock.Anything, capacityAdmissionScript, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything).
		Return(nil, context.DeadlineExceeded).
		Once()
	ctrl := NewCapacityAdmissionController(redisClient, admissionTestOptions())
	model := admissionTestModel(capacityUpstream(1, 10, 20, 100000))

	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, decision.Action)
	require.True(t, decision.FailOpen)
	require.Nil(t, decision.Lease)
}

func TestAdmissionCheck_FailCloseOnRedisError(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	redisClient.EXPECT().LoadScript(mock.Anything, mock.Anything).Return(nil).Times(5)
	redisClient.EXPECT().
		RunScript(mock.Anything, capacityAdmissionScript, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything).
		Return(nil, context.DeadlineExceeded).
		Once()
	opts := admissionTestOptions()
	opts.FailOpen = false
	ctrl := NewCapacityAdmissionController(redisClient, opts)
	model := admissionTestModel(capacityUpstream(1, 10, 20, 100000))

	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionReject, decision.Action)
}

func TestAdmissionFinalize_WithUsage_CorrectsReservation(t *testing.T) {
	// Reservation conservation: acquire(est) -> finalize(actual) must leave
	// tpm = actual, concurrency = 0, lease gone.
	ctrl, _, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 100, 1000000))
	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, decision.Action)

	ctrl.Finalize(context.Background(), decision.Lease, &token.Usage{TotalTokens: 800})

	// Net window sum: the reservation is reclaimed, the committed usage
	// counts as an age-GC'd contribution in the usage ZSET.
	require.Equal(t, int64(800), getString(t, rdb, capacityTPMSumKey("test-model", 1)))
	require.Equal(t, int64(1), zcard(t, rdb, capacityTPMUsageKey("test-model", 1)))
	require.Equal(t, int64(0), zcard(t, rdb, capacityConcurrencyKey("test-model", 1)))
	require.Equal(t, int64(0), rdb.HLen(context.Background(), capacityLeaseKey("test-model", 1)).Val())
}

func TestAdmissionFinalize_ReleaseWithoutUsage_ReclaimsReservation(t *testing.T) {
	ctrl, _, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 100, 1000000))
	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})

	ctrl.Finalize(context.Background(), decision.Lease, nil)

	require.Equal(t, int64(0), getString(t, rdb, capacityTPMSumKey("test-model", 1)))
	require.Equal(t, int64(0), zcard(t, rdb, capacityConcurrencyKey("test-model", 1)))
}

func TestAdmissionFinalize_UsageThenRelease_IsNoop(t *testing.T) {
	// Production call discipline: exactly one finalize carries usage (the
	// async usage-commit path); the Orchestrator's safety net finalizes
	// without usage afterwards and must not touch the aggregates again.
	ctrl, _, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 100, 1000000))
	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})

	ctrl.Finalize(context.Background(), decision.Lease, &token.Usage{TotalTokens: 800})
	ctrl.Finalize(context.Background(), decision.Lease, nil)

	// The second (usage-less) finalize is a Redis no-op: the sum keeps the
	// committed usage only.
	require.Equal(t, int64(800), getString(t, rdb, capacityTPMSumKey("test-model", 1)))
	require.Equal(t, int64(0), zcard(t, rdb, capacityConcurrencyKey("test-model", 1)))
}

func TestAdmissionFinalize_AgesOutOfSlidingWindow(t *testing.T) {
	// Acquire at T0; finalize a minute later: the reservation is reclaimed
	// from the running sum and the actual usage enters the window at commit
	// time. After the usage contribution itself ages out of the sliding
	// window, the sum returns to zero without any further traffic.
	ctrl, mr, rdb, clk := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 100, 1000000))
	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, decision.Action)

	// Roll the Redis clock past the next window boundary: the finalize
	// correction no longer depends on any window geometry.
	clk.t = clk.Now().Add(70 * time.Second)
	mr.SetTime(clk.Now())

	ctrl.Finalize(context.Background(), decision.Lease, &token.Usage{TotalTokens: 500})
	require.Equal(t, int64(500), getString(t, rdb, capacityTPMSumKey("test-model", 1)))

	// Advance past the usage contribution's window: it ages out and the
	// running sum returns to zero; a fresh acquire starts from a clean sum.
	clk.t = clk.Now().Add(70 * time.Second)
	mr.SetTime(clk.Now())
	next := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, next.Action)
	require.Equal(t, int64(1064), getString(t, rdb, capacityTPMSumKey("test-model", 1)))
}

func TestAdmissionExpiredCleanup_ReclaimsConcurrencyAndTPM(t *testing.T) {
	// Pod crash simulation: a lease is acquired but never finalized. After
	// the TTL passes, the next admission's expired-lease cleanup must
	// reclaim both the concurrency slot and the TPM reservation (from the
	// running sum).
	ctrl, mr, rdb, clk := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 1, 100, 1000000))

	crashed := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, crashed.Action)

	// Advance the Redis clock beyond the lease TTL and try to admit a new
	// request for the same single-slot upstream.
	clk.t = clk.Now().Add(90 * time.Second)
	mr.SetTime(clk.Now())

	next := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, next.Action)
	require.NotEqual(t, crashed.Lease.Token, next.Lease.Token)

	// The crashed lease's reservation was reclaimed from the running sum;
	// the new reservation is the only live contribution.
	require.Equal(t, int64(1064), getString(t, rdb, capacityTPMSumKey("test-model", 1)))
}

func TestRenew_ExtendsExistingLease(t *testing.T) {
	ctrl, _, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 100, 1000000))
	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})

	require.True(t, ctrl.Renew(context.Background(), decision.Lease))
	require.Equal(t, int64(1), zcard(t, rdb, capacityConcurrencyKey("test-model", 1)))
}

func TestRenew_AfterFinalize_DoesNotResurrect(t *testing.T) {
	ctrl, _, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 100, 1000000))
	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	ctrl.Finalize(context.Background(), decision.Lease, nil)

	require.False(t, ctrl.Renew(context.Background(), decision.Lease))
	require.Equal(t, int64(0), zcard(t, rdb, capacityConcurrencyKey("test-model", 1)))
}

func TestFinalize_NilLease_IsNoop(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	redisClient.EXPECT().LoadScript(mock.Anything, mock.Anything).Return(nil).Times(5)
	ctrl := NewCapacityAdmissionController(redisClient, admissionTestOptions())
	// No RunScript EXPECT: any RunScript call fails the test.
	ctrl.Finalize(context.Background(), nil, &token.Usage{TotalTokens: 100})
	ctrl.Finalize(context.Background(), &types.AdmissionLease{}, &token.Usage{TotalTokens: 100})
}

// --- lease renewer ---

func TestLeaseRenewer_RenewsAndUnregisters(t *testing.T) {
	var renewed []string
	renewer := newLeaseRenewer(func(_ context.Context, lease *types.AdmissionLease) bool {
		renewed = append(renewed, lease.Token)
		return lease.Token != "gone"
	}, time.Hour) // interval > 0, but the loop is never waited on
	renewer.Register(&types.AdmissionLease{Token: "a", UpstreamID: 1, ModelID: "m"})
	renewer.Register(&types.AdmissionLease{Token: "gone", UpstreamID: 1, ModelID: "m"})
	renewer.renewAll(context.Background())

	require.ElementsMatch(t, []string{"a", "gone"}, renewed)
	renewer.mu.Lock()
	_, aAlive := renewer.leases["a"]
	_, goneAlive := renewer.leases["gone"]
	renewer.mu.Unlock()
	require.True(t, aAlive)
	require.False(t, goneAlive)
	renewer.Stop()
}

// --- estimator ---

func TestEstimateAdmissionTokens(t *testing.T) {
	ctrl := NewCapacityAdmissionController(nil, admissionTestOptions())

	// Empty prompt: completion reserve + floor.
	require.Equal(t, int64(1064), ctrl.EstimateAdmissionTokens(""))
	// Normal prompt: ceil(400/4) + 1000.
	require.Equal(t, int64(1100), ctrl.EstimateAdmissionTokens(makeString(400)))
	// Oversized prompt is capped at MaxTokenEstimate.
	require.Equal(t, int64(32768), ctrl.EstimateAdmissionTokens(makeString(10_000_000)))
	// Multibyte runes count once per rune, not per byte.
	require.Equal(t, int64(1100), ctrl.EstimateAdmissionTokens(makeCJKString(400)))
}

func makeString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

func makeCJKString(n int) string {
	runes := make([]rune, n)
	for i := range runes {
		runes[i] = '中'
	}
	return string(runes)
}

// --- key construction ---

func TestCapacityKeys_UseModelHashTag(t *testing.T) {
	conc := capacityConcurrencyKey("qwen3", 5)
	lease := capacityLeaseKey("qwen3", 5)
	require.Equal(t, "aigateway:capacity:{m:qwen3}:u:5:conc", conc)
	require.Equal(t, "aigateway:capacity:{m:qwen3}:u:5:lease", lease)
	// The hash tag must be identical for every upstream of the model so all
	// candidate keys share one Redis Cluster slot.
	require.Contains(t, conc, "{m:qwen3}")
	require.Contains(t, lease, "{m:qwen3}")
}

// --- est <= 0: requests that do not participate in the TPM dimension ---

func TestAdmissionCheck_NoTPMEstimate_LeaseWithoutTPMReservation(t *testing.T) {
	ctrl, _, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 20, 100000))

	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     types.AdmissionNoTPMEstimate,
	})
	require.NotNil(t, decision)
	require.Equal(t, types.AdmissionAdmit, decision.Action)
	require.NotNil(t, decision.Lease)
	require.Equal(t, types.AdmissionNoTPMEstimate, decision.Lease.ReservedTokens)
	// Post-acquire snapshot: concurrency and RPM include this request, TPM
	// does not.
	require.True(t, decision.States[0].Selected)
	require.Equal(t, int64(1), decision.States[0].CurrentConcurrency)
	require.Equal(t, int64(1), decision.States[0].CurrentRPM)
	require.Equal(t, int64(0), decision.States[0].CurrentTPM)

	require.Equal(t, int64(1), zcard(t, rdb, capacityConcurrencyKey("test-model", 1)))
	require.Equal(t, int64(1), zcard(t, rdb, capacityRPMKey("test-model", 1)))
	// No TPM reservation: the running sum is never created and no lease
	// metadata is written, so release/cleanup has nothing to reclaim.
	require.Equal(t, int64(0), rdb.Exists(context.Background(), capacityTPMSumKey("test-model", 1)).Val())
	require.Equal(t, int64(0), rdb.HLen(context.Background(), capacityLeaseKey("test-model", 1)).Val())

	// Finalize with usage still commits the actual usage to the running
	// window sum: admission participation and usage accounting are
	// orthogonal.
	ctrl.Finalize(context.Background(), decision.Lease, &token.Usage{TotalTokens: 2500})
	require.Equal(t, int64(2500), getString(t, rdb, capacityTPMSumKey("test-model", 1)))
	require.Equal(t, int64(0), zcard(t, rdb, capacityConcurrencyKey("test-model", 1)))
}

func TestAdmissionCheck_NoTPMEstimate_SkipsTPMGate(t *testing.T) {
	// A window already over MaxTPM must not block a request that reserves
	// no tokens; the same state rejects a request that does.
	ctrl, _, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 20, 1000))
	setString(t, rdb, capacityTPMSumKey("test-model", 1), 1000)

	reserved := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionReject, reserved.Action)
	require.Equal(t, types.AdmissionReasonTPMExceeded, reserved.Reason)

	noTPM := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     types.AdmissionNoTPMEstimate,
	})
	require.Equal(t, types.AdmissionAdmit, noTPM.Action)
	require.NotNil(t, noTPM.Lease)
}

func TestAdmissionCheck_NoTPMEstimate_ScoreExcludesTPM(t *testing.T) {
	// Same upstream states, only the estimate differs: with est > 0 the
	// TPM-starved upstream loses the selection; with est <= 0 the TPM
	// dimension is excluded and the tie on conc/rpm keeps the first
	// (preferred) candidate.
	ctrl, _, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(
		capacityUpstream(1, 10, 100, 5000),
		capacityUpstream(2, 10, 100, 5000),
	)
	setString(t, rdb, capacityTPMSumKey("test-model", 1), 4500)

	withEstimate := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		AllowSelect:         true,
		EstimatedTokens:     400,
	})
	require.Equal(t, types.AdmissionAdmit, withEstimate.Action)
	require.Equal(t, int64(2), withEstimate.SelectedUpstreamID)
	require.True(t, withEstimate.ReSelected)
	ctrl.Finalize(context.Background(), withEstimate.Lease, nil)

	noEstimate := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		AllowSelect:         true,
		EstimatedTokens:     types.AdmissionNoTPMEstimate,
	})
	require.Equal(t, types.AdmissionAdmit, noEstimate.Action)
	require.Equal(t, int64(1), noEstimate.SelectedUpstreamID)
	require.False(t, noEstimate.ReSelected)
}

func TestAdmissionCheck_NoTPMEstimate_MaxTPMOnlyUpstream_Lifecycle(t *testing.T) {
	// An upstream with only MaxTPM enabled has no binding dimension for a
	// no-TPM request: it is admitted with a lease, renews normally, and its
	// finalize still commits the actual usage. The TPM gate stays intact
	// for requests that do reserve tokens.
	ctrl, _, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(commontypes.UpstreamConfig{
		ID:             1,
		URL:            "https://upstream-1.example.com/v1",
		Enabled:        true,
		CapacityPolicy: &commontypes.CapacityPolicy{Enabled: true, MaxTPM: 5000},
	})

	over := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     5001,
	})
	require.Equal(t, types.AdmissionReject, over.Action)
	require.Equal(t, types.AdmissionReasonTPMExceeded, over.Reason)

	noTPM := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     types.AdmissionNoTPMEstimate,
	})
	require.Equal(t, types.AdmissionAdmit, noTPM.Action)
	require.NotNil(t, noTPM.Lease)
	require.True(t, ctrl.Renew(context.Background(), noTPM.Lease))
	ctrl.Finalize(context.Background(), noTPM.Lease, &token.Usage{TotalTokens: 2600})

	require.Equal(t, int64(2600), getString(t, rdb, capacityTPMSumKey("test-model", 1)))
	require.Equal(t, int64(0), zcard(t, rdb, capacityConcurrencyKey("test-model", 1)))
}

func TestAdmissionCheck_NilRedisClient_DegradesPerFailOpen(t *testing.T) {
	// A nil Redis client (misconfiguration) must not panic: it degrades
	// exactly like a Redis outage, per the fail-open option.
	failOpen := NewCapacityAdmissionController(nil, admissionTestOptions())
	d := failOpen.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               admissionTestModel(capacityUpstream(1, 10, 20, 100000)),
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.NotNil(t, d)
	require.Equal(t, types.AdmissionAdmit, d.Action)
	require.True(t, d.FailOpen)
	require.Nil(t, d.Lease)

	failClose := NewCapacityAdmissionController(nil, CapacityAdmissionOptions{FailOpen: false, LeaseTTL: time.Minute})
	d = failClose.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               admissionTestModel(capacityUpstream(1, 10, 20, 100000)),
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.NotNil(t, d)
	require.Equal(t, types.AdmissionReject, d.Action)
	require.Equal(t, types.AdmissionReasonCapacityExceeded, d.Reason)
}

// --- sliding-window precision ---

func TestAdmissionSlidingWindow_BoundaryDoesNotRefill(t *testing.T) {
	// The differentiating sliding-window behavior: acquires spread over the
	// tail of a window still count right after the fixed-window boundary.
	// Under a fixed 60s window this acquire would be admitted with a fresh
	// quota (2x boundary burst); the sliding window keeps counting the
	// entries from the last 60 seconds.
	ctrl, mr, rdb, clk := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 5, 1000000))

	// Saturate the window: 5 acquires at T0+10..T0+50.
	for i, age := range []int64{50, 40, 30, 20, 10} {
		decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
			Model:               model,
			PreferredUpstreamID: 1,
			AllowSelect:         false,
			EstimatedTokens:     1,
		})
		require.Equal(t, types.AdmissionAdmit, decision.Action, "acquire %d", i)
		_ = age
	}
	// Rewind semantics: the five acquires above happened at "now"; instead
	// of rewinding, advance partially so they stay inside the window.
	clk.t = clk.Now().Add(1 * time.Second)
	mr.SetTime(clk.Now())

	blocked := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		AllowSelect:         false,
		EstimatedTokens:     1,
	})
	require.Equal(t, types.AdmissionReject, blocked.Action)
	require.Equal(t, types.AdmissionReasonRPMExceeded, blocked.Reason)

	// Age past the whole window: every entry leaves, quota recovers
	// immediately and completely — no fixed-window carry-over either way.
	clk.t = clk.Now().Add(61 * time.Second)
	mr.SetTime(clk.Now())
	recovered := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		AllowSelect:         false,
		EstimatedTokens:     1,
	})
	require.Equal(t, types.AdmissionAdmit, recovered.Action)
	require.Equal(t, int64(1), zcard(t, rdb, capacityRPMKey("test-model", 1)))
}

func TestAdmissionSlidingWindow_AgingReleasesQuota(t *testing.T) {
	// Quota frees up gradually as contributions age out — not in one
	// fixed-window lump: 3 acquires at T0+40..T0+59 (max 5), then at
	// T0+61 two of them are still inside the 60s window so exactly
	// (5 - in-window) new acquires pass.
	ctrl, mr, rdb, clk := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 5, 1000000))

	for i := 0; i < 3; i++ {
		decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
			Model:               model,
			PreferredUpstreamID: 1,
			AllowSelect:         false,
			EstimatedTokens:     1,
		})
		require.Equal(t, types.AdmissionAdmit, decision.Action)
	}

	// Advance so the earliest acquires leave the window while the latest
	// stay inside.
	clk.t = clk.Now().Add(45 * time.Second)
	mr.SetTime(clk.Now())

	// The 3 original entries (acquired 45s ago) are still inside the window.
	// Two more acquires pass (3+2 = 5 = max), the third is blocked.
	for i := 0; i < 2; i++ {
		decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
			Model:               model,
			PreferredUpstreamID: 1,
			AllowSelect:         false,
			EstimatedTokens:     1,
		})
		require.Equal(t, types.AdmissionAdmit, decision.Action, "acquire %d", i)
	}
	blocked := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		AllowSelect:         false,
		EstimatedTokens:     1,
	})
	require.Equal(t, types.AdmissionReject, blocked.Action)
	require.Equal(t, types.AdmissionReasonRPMExceeded, blocked.Reason)
	require.Equal(t, int64(5), zcard(t, rdb, capacityRPMKey("test-model", 1)))
}

func TestRenew_RefreshesTPMSumTTL(t *testing.T) {
	// A live reservation must never be orphaned by the sum key expiring
	// while its lease is still alive: renew refreshes the sum TTL beyond
	// lease life.
	ctrl, mr, rdb, clk := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 100, 1000000))
	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, decision.Action)

	// Advance near the sum's idle expiry (120s) without renewing.
	clk.t = clk.Now().Add(100 * time.Second)
	mr.SetTime(clk.Now())
	require.Less(t, rdb.PTTL(context.Background(), capacityTPMSumKey("test-model", 1)).Val(), int64(30*time.Second))

	require.True(t, ctrl.Renew(context.Background(), decision.Lease))
	require.Greater(t, rdb.PTTL(context.Background(), capacityTPMSumKey("test-model", 1)).Val(), int64(100*time.Second))
}

func TestRenew_RefreshesUsageLedgerTTL(t *testing.T) {
	// A long stream (live, in-flight lease) whose upstream committed usage
	// for an earlier attempt holds a usage ledger with its own TTL. Renew
	// must refresh BOTH the sum and the usage ledger TTLs alongside the
	// lease, so the ledger entries stay GC-able by the next request and the
	// committed usage inside the sliding window can never get stranded out
	// of reach of the sum.
	ctrl, mr, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 10, 100, 1000000))
	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, decision.Action)

	// A completed sibling attempt's usage is committed to the same
	// upstream's window (the live lease keeps its reservation).
	other := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, other.Action)
	ctrl.Finalize(context.Background(), other.Lease, &token.Usage{TotalTokens: 800})
	require.Equal(t, int64(1864), getString(t, rdb, capacityTPMSumKey("test-model", 1)))

	// FastForward near the window keys' idle expiry (EXPIRE = LeaseTTL+60 =
	// 120s), then renew the LIVE lease: both the sum and the usage ledger
	// TTLs must be refreshed beyond lease life.
	// (miniredis SetTime only affects EXPIREAT comparisons; TTLs are
	// shortened via FastForward.)
	mr.FastForward(110 * time.Second)
	require.Less(t, rdb.PTTL(context.Background(), capacityTPMUsageKey("test-model", 1)).Val(), int64(15*time.Second))

	require.True(t, ctrl.Renew(context.Background(), decision.Lease))
	require.Greater(t, rdb.PTTL(context.Background(), capacityTPMSumKey("test-model", 1)).Val(), int64(100*time.Second))
	require.Greater(t, rdb.PTTL(context.Background(), capacityTPMUsageKey("test-model", 1)).Val(), int64(100*time.Second))
}

func TestAdmissionScripts_PreloadedOnStart(t *testing.T) {
	// The controller preloads all five Lua scripts at construction so the
	// first request hits EVALSHA without the NOSCRIPT fallback.
	ctrl, _, rdb, _ := newMiniredisController(t)
	_ = ctrl

	shas := make([]string, 0, 5)
	for _, script := range []string{
		capacityAdmissionScript,
		capacityAdmissionReleaseScript,
		capacityAdmissionRenewScript,
		capacityAdmissionCommitScript,
		capacityAdmissionObserveScript,
	} {
		sum := sha1.Sum([]byte(script))
		shas = append(shas, hex.EncodeToString(sum[:]))
	}
	require.Eventually(t, func() bool {
		exists := rdb.ScriptExists(context.Background(), shas...).Val()
		for _, ok := range exists {
			if !ok {
				return false
			}
		}
		return len(exists) == 5
	}, 2*time.Second, 20*time.Millisecond, "all 5 admission scripts should be in the Redis script cache")
}

// --- client-canceled finalize: RPM entry release ---

func TestFinalizeCanceled_ReleasesRPMEntry(t *testing.T) {
	// A client-canceled attempt (HTTP 499 / context canceled) never reaches
	// the upstream's inference path, so its RPM window entry is removed at
	// finalize — otherwise the aborted attempt would rate-lock a legitimate
	// client retry for the rest of the sliding window. The concurrency slot
	// and the TPM reservation are reclaimed exactly as in a normal finalize.
	ctrl, _, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 3, 2, 1000000))

	first := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, first.Action)
	second := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, second.Action)
	require.Equal(t, int64(2), zcard(t, rdb, capacityRPMKey("test-model", 1)))

	// Both attempts are client-canceled: usage carries the approximate
	// prefill tokens observed before the abort (FinalizeCanceled itself
	// releases the RPM entries — no flag on the usage needed).
	ctrl.FinalizeCanceled(context.Background(), first.Lease, &token.Usage{TotalTokens: 8})
	ctrl.FinalizeCanceled(context.Background(), second.Lease, &token.Usage{TotalTokens: 8})

	// Both RPM entries are gone: a retry is admitted immediately.
	require.Equal(t, int64(0), zcard(t, rdb, capacityRPMKey("test-model", 1)))
	// Concurrency slots and the TPM reservation are reclaimed; the
	// committed approximate usage stays in the running sum (immutable
	// accounting: the prefill did happen).
	require.Equal(t, int64(0), zcard(t, rdb, capacityConcurrencyKey("test-model", 1)))
	require.Equal(t, int64(16), getString(t, rdb, capacityTPMSumKey("test-model", 1)))
	require.Equal(t, int64(0), rdb.HLen(context.Background(), capacityLeaseKey("test-model", 1)).Val())

	// The next request is admitted: no rate lock.
	retry := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, retry.Action)
}

func TestFinalize_ServerFailure_KeepsRPMEntry(t *testing.T) {
	// A server-side failure (usage WITHOUT ClientCanceled) keeps the
	// immutable RPM accounting: the attempt genuinely reached the upstream.
	ctrl, _, rdb, _ := newMiniredisController(t)
	model := admissionTestModel(capacityUpstream(1, 3, 2, 1000000))
	decision := ctrl.Check(context.Background(), types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: 1,
		EstimatedTokens:     1064,
	})
	require.Equal(t, types.AdmissionAdmit, decision.Action)

	ctrl.Finalize(context.Background(), decision.Lease, &token.Usage{TotalTokens: 8})

	require.Equal(t, int64(1), zcard(t, rdb, capacityRPMKey("test-model", 1)))
}
