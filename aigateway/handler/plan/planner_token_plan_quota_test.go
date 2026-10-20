package plan

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

// mockTokenPlanQuotaChecker records the request of the plan-phase quota check
// and returns a canned error.
type mockTokenPlanQuotaChecker struct {
	err        error
	called     bool
	gotNsUUID  string
	gotModelID string
}

func (m *mockTokenPlanQuotaChecker) CheckTokenPlanQuota(_ context.Context, req commontypes.TokenPlanQuotaReq) error {
	m.called = true
	m.gotNsUUID = req.NsUUID
	m.gotModelID = req.ModelID
	return m.err
}

func tokenPlanPlannerDeps(checker TokenPlanQuotaChecker) PlannerDeps {
	return PlannerDeps{
		ModelResolver:         &mockModelResolver{target: makeResolvedTarget("http://upstream/v1/messages", "")},
		BalanceChecker:        &mockBalanceChecker{},
		ContentSafety:         &mockContentSafetyChecker{},
		TokenPlanQuotaChecker: checker,
	}
}

func tokenPlanMeta(tenantID string) *types.RequestMetadata {
	return &types.RequestMetadata{
		Protocol: string(types.ProtocolMessages),
		Task:     "messages",
		UserID:   "user1",
		Model:    "test-model",
		TenantID: tenantID,
	}
}

func TestPlan_TokenPlanQuotaExceeded(t *testing.T) {
	checker := &mockTokenPlanQuotaChecker{
		err: &types.TokenPlanQuotaExceededError{ModelID: "test-model", UsedTokens: 120, QuotaTokens: 100},
	}

	plan, err := NewPlanner(tokenPlanPlannerDeps(checker)).Plan(newTestGinContext(), tokenPlanMeta("ns-123"))

	require.Error(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, types.PlanErrQuotaExceeded, plan.ErrorCode)
	assert.True(t, checker.called)
	assert.Equal(t, "ns-123", checker.gotNsUUID)
	assert.Equal(t, "test-model", checker.gotModelID)
	// The quota gate rejects before balance, so a rejected request is never
	// recorded as balance-checked.
	assert.False(t, plan.BalanceOK)
}

func TestPlan_TokenPlanQuotaInfraFailureIsInternal(t *testing.T) {
	checker := &mockTokenPlanQuotaChecker{err: errors.New("accounting unavailable")}

	plan, err := NewPlanner(tokenPlanPlannerDeps(checker)).Plan(newTestGinContext(), tokenPlanMeta("ns-123"))

	require.Error(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, types.PlanErrInternal, plan.ErrorCode,
		"an accounting outage must not be reported as quota exhaustion")
}

// TestPlan_TokenPlanQuotaUsesResolvedModelID proves the lookup is keyed on the
// model the resolver canonicalized to, not the raw request string: an alias or
// legacy spelling must still enforce the canonical model's allowance.
func TestPlan_TokenPlanQuotaUsesResolvedModelID(t *testing.T) {
	checker := &mockTokenPlanQuotaChecker{}
	resolver := &mockModelResolver{target: makeResolvedTarget("http://upstream/v1/chat/completions", "")}
	resolver.target.Model.ID = "canonical-model"
	resolver.target.ModelName = "canonical-model"
	deps := PlannerDeps{
		ModelResolver:         resolver,
		BalanceChecker:        &mockBalanceChecker{},
		ContentSafety:         &mockContentSafetyChecker{},
		TokenPlanQuotaChecker: checker,
	}
	meta := tokenPlanMeta("ns-123")
	meta.Model = "legacy-alias"

	plan, err := NewPlanner(deps).Plan(newTestGinContext(), meta)

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.True(t, checker.called)
	assert.Equal(t, "canonical-model", checker.gotModelID,
		"the quota must be looked up by the resolved model ID, not the raw request string")
}

// TestPlan_TokenPlanQuotaFallsBackToRequestedModelID covers the abnormal case
// where resolution leaves the resolved ID unset: the requested ID still gates
// the request instead of silently skipping the quota.
func TestPlan_TokenPlanQuotaFallsBackToRequestedModelID(t *testing.T) {
	checker := &mockTokenPlanQuotaChecker{}
	resolver := &mockModelResolver{target: makeResolvedTarget("http://upstream/v1/chat/completions", "")}
	resolver.target.Model.ID = ""
	deps := PlannerDeps{
		ModelResolver:         resolver,
		BalanceChecker:        &mockBalanceChecker{},
		ContentSafety:         &mockContentSafetyChecker{},
		TokenPlanQuotaChecker: checker,
	}
	meta := tokenPlanMeta("ns-123")
	meta.Model = "requested-model"

	_, err := NewPlanner(deps).Plan(newTestGinContext(), meta)

	require.NoError(t, err)
	assert.True(t, checker.called)
	assert.Equal(t, "requested-model", checker.gotModelID)
}

func TestPlan_TokenPlanQuotaSkippedWithoutTenant(t *testing.T) {
	checker := &mockTokenPlanQuotaChecker{err: errors.New("must not be called")}

	plan, err := NewPlanner(tokenPlanPlannerDeps(checker)).Plan(newTestGinContext(), tokenPlanMeta(""))

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.False(t, checker.called, "a request without a namespace cannot be attributed to a subscription")
	assert.True(t, plan.BalanceOK)
}
