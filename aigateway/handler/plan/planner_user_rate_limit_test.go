package plan

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

// mockUserRateLimitChecker records CheckUserRateLimit/RecordUserRateLimit
// calls and returns the configured check error.
type mockUserRateLimitChecker struct {
	err      error
	calls    int
	lastMeta *types.RequestMetadata
}

func (m *mockUserRateLimitChecker) CheckUserRateLimit(ctx context.Context, meta *types.RequestMetadata, mt *types.ModelTarget) error {
	m.calls++
	m.lastMeta = meta
	return m.err
}

func rateLimitPlanMetadata() *types.RequestMetadata {
	return &types.RequestMetadata{
		Protocol: string(types.ProtocolMessages),
		Task:     "messages",
		UserID:   "user1",
		Model:    "test-model",
		TenantID: "ns-123",
	}
}

func TestPlan_UserRateLimitExceeded_Rejected(t *testing.T) {
	checker := &mockUserRateLimitChecker{err: &types.UserRateLimitExceededError{
		Scope:             types.UserRateLimitScopeGlobal,
		Dimension:         types.UserRateLimitDimensionRPM,
		RetryAfterSeconds: 42,
	}}
	p := NewPlanner(PlannerDeps{
		ModelResolver:        &mockModelResolver{target: makeResolvedTarget("http://upstream/v1/messages", "")},
		BalanceChecker:       &mockBalanceChecker{},
		ContentSafety:        &mockContentSafetyChecker{},
		UserRateLimitChecker: checker,
	})

	plan, err := p.Plan(newTestGinContext(), rateLimitPlanMetadata())
	require.Error(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, types.PlanErrUserRateLimitExceeded, plan.ErrorCode)
	assert.Equal(t, 1, checker.calls)
}

func TestPlan_UserRateLimit_NonLimitErrorIsInternal(t *testing.T) {
	checker := &mockUserRateLimitChecker{err: errors.New("unexpected checker failure")}
	p := NewPlanner(PlannerDeps{
		ModelResolver:        &mockModelResolver{target: makeResolvedTarget("http://upstream/v1/messages", "")},
		BalanceChecker:       &mockBalanceChecker{},
		ContentSafety:        &mockContentSafetyChecker{},
		UserRateLimitChecker: checker,
	})

	plan, err := p.Plan(newTestGinContext(), rateLimitPlanMetadata())
	require.Error(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, types.PlanErrInternal, plan.ErrorCode)
}

func TestPlan_UserRateLimit_NilCheckerSkips(t *testing.T) {
	p := NewPlanner(PlannerDeps{
		ModelResolver:  &mockModelResolver{target: makeResolvedTarget("http://upstream/v1/messages", "")},
		BalanceChecker: &mockBalanceChecker{},
		ContentSafety:  &mockContentSafetyChecker{},
	})

	plan, err := p.Plan(newTestGinContext(), rateLimitPlanMetadata())
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.True(t, plan.BalanceOK)
}

func TestPlan_UserRateLimit_NoTenant_SkipsCheck(t *testing.T) {
	checker := &mockUserRateLimitChecker{}
	p := NewPlanner(PlannerDeps{
		ModelResolver:        &mockModelResolver{target: makeResolvedTarget("http://upstream/v1/messages", "")},
		BalanceChecker:       &mockBalanceChecker{},
		ContentSafety:        &mockContentSafetyChecker{},
		UserRateLimitChecker: checker,
	})

	meta := rateLimitPlanMetadata()
	meta.TenantID = ""
	_, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	// The component (not the planner) owns the no-tenant short circuit; the
	// checker still receives the call and must ignore it.
	assert.Equal(t, 1, checker.calls)
}

func TestPlan_UserRateLimit_RerouteRePlanCountsOnce(t *testing.T) {
	// An admission-queue reroute re-runs the whole plan; the per-user rate
	// limit check must NOT re-run with it — the request was already counted
	// on the first round.
	checker := &mockUserRateLimitChecker{}
	p := NewPlanner(PlannerDeps{
		ModelResolver:        &mockModelResolver{target: admissionPlanTestTarget()},
		BalanceChecker:       &mockBalanceChecker{},
		ContentSafety:        &mockContentSafetyChecker{},
		AdmissionChecker:     &rerouteAdmissionChecker{afterCall: &types.AdmissionOutcome{Decision: &types.AdmissionDecision{Action: types.AdmissionAdmit}}},
		UserRateLimitChecker: checker,
	})

	plan, err := p.Plan(newTestGinContext(), rateLimitPlanMetadata())
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, types.AdmissionAdmit, plan.Admission.Action)
	assert.Equal(t, 1, checker.calls, "rate limit must be checked once per request, not per plan round")
}

// The check-and-consume contract: the user RPM gate runs exactly once per
// request BEFORE capacity admission — whether the request is later admitted,
// capacity-rejected or queued does not re-run it, and the consumption is
// final either way (deliberate product trade-off: charging every attempt
// against a saturated upstream damps retry storms).
func TestPlan_UserRateLimit_CapacityReject_StillConsumedOnce(t *testing.T) {
	checker := &mockUserRateLimitChecker{}
	p := NewPlanner(PlannerDeps{
		ModelResolver:        &mockModelResolver{target: admissionPlanTestTarget()},
		BalanceChecker:       &mockBalanceChecker{},
		ContentSafety:        &mockContentSafetyChecker{},
		AdmissionChecker:     &recordingAdmissionChecker{outcome: &types.AdmissionOutcome{Decision: &types.AdmissionDecision{Action: types.AdmissionReject}}, safety: &recordingSafetyChecker{}},
		UserRateLimitChecker: checker,
	})

	plan, err := p.Plan(newTestGinContext(), rateLimitPlanMetadata())
	require.Error(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, types.PlanErrCapacityExceeded, plan.ErrorCode)
	assert.Equal(t, 1, checker.calls, "the gate runs once, before admission")
}

func TestPlan_UserRateLimit_NoAdmission_CheckStillRuns(t *testing.T) {
	// Admission not applicable (no CapacityPolicy): the user gate still
	// runs and consumes.
	checker := &mockUserRateLimitChecker{}
	p := NewPlanner(PlannerDeps{
		ModelResolver:        &mockModelResolver{target: makeResolvedTarget("http://upstream/v1/messages", "")},
		BalanceChecker:       &mockBalanceChecker{},
		ContentSafety:        &mockContentSafetyChecker{},
		UserRateLimitChecker: checker,
	})

	_, err := p.Plan(newTestGinContext(), rateLimitPlanMetadata())
	require.NoError(t, err)
	assert.Equal(t, 1, checker.calls)
}
