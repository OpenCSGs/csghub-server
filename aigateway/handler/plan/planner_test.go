package plan

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/handler/protocol"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/common/errorx"
	commonType "opencsg.com/csghub-server/common/types"
)

// --- Mock implementations ---

type mockModelResolver struct {
	target *types.ModelTarget
	err    error
}

func (m *mockModelResolver) ResolveModelTarget(ctx context.Context, username, modelID string, headers http.Header, opts ResolveOptions) (*types.ModelTarget, error) {
	return m.target, m.err
}

type mockBalanceChecker struct {
	err error
}

func (m *mockBalanceChecker) CheckBalance(ctx context.Context, nsUUID string) error {
	return m.err
}

type mockUsageLimitChecker struct {
	err error
}

func (m *mockUsageLimitChecker) CheckUsageLimit(ctx context.Context, nsUUID string, model *types.Model, endpoint string) error {
	return m.err
}

type mockContentSafetyChecker struct {
	isSensitive bool
	message     string
	err         error
}

func (m *mockContentSafetyChecker) Check(ctx context.Context, model *types.Model, promptText, tenantID, task string, streaming bool, provider string) (bool, string, error) {
	// Mirror the real adapter: non-token tasks skip safety entirely.
	switch task {
	case "chat", "responses", "messages":
	default:
		return false, "", nil
	}
	return m.isSensitive, m.message, m.err
}

// codedErrStub is a minimal CodedError implementation for categorizePlanError tests.
type codedErrStub struct {
	code string
}

func (e *codedErrStub) Error() string         { return e.code }
func (e *codedErrStub) ModelErrorCode() string { return e.code }

func makeResolvedTarget(targetURL, upstreamProtocol string) *types.ModelTarget {
	return &types.ModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
		},
		Upstream: commonType.UpstreamConfig{
			URL:      targetURL,
			Provider: "test",
		},
		Target:    targetURL,
		ModelName: "test-model",
	}
}

type promptTextProvider struct {
	text string
}

func (p *promptTextProvider) PromptText() string { return p.text }

// stubMetricsEnricher records the last SetModelTarget call for assertions.
type stubMetricsEnricher struct {
	called   bool
	modelID  string
	target   *types.ModelTarget
	isStream bool
}

func (s *stubMetricsEnricher) SetModelTarget(c *gin.Context, modelID string, target *types.ModelTarget, isStream bool) {
	s.called = true
	s.modelID = modelID
	s.target = target
	s.isStream = isStream
}

// newTestGinContext creates a minimal gin.Context suitable for calling
// Planner.Plan in unit tests.
func newTestGinContext() *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)
	return c
}

// --- categorizePlanError tests ---

func TestCategorizePlanError_Nil(t *testing.T) {
	assert.Equal(t, types.PlanErrUnknown, categorizePlanError(nil))
}

func TestCategorizePlanError_ModelNotFound(t *testing.T) {
	err := &codedErrStub{code: "model_not_found"}
	assert.Equal(t, types.PlanErrModelNotFound, categorizePlanError(err))
}

func TestCategorizePlanError_ModelNotRunning(t *testing.T) {
	err := &codedErrStub{code: "model_not_running"}
	assert.Equal(t, types.PlanErrModelNotFound, categorizePlanError(err))
}

func TestCategorizePlanError_ModelUnavailable(t *testing.T) {
	err := &codedErrStub{code: "model_unavailable"}
	assert.Equal(t, types.PlanErrModelUnavailable, categorizePlanError(err))
}

func TestCategorizePlanError_RequiredUpstreamUnavailable(t *testing.T) {
	err := &codedErrStub{code: "required_upstream_unavailable"}
	assert.Equal(t, types.PlanErrModelUnavailable, categorizePlanError(err))
}

func TestCategorizePlanError_InsufficientBalance(t *testing.T) {
	err := errorx.ErrInsufficientBalance
	assert.Equal(t, types.PlanErrInsufficientBalance, categorizePlanError(err))
}

func TestCategorizePlanError_WrappedInsufficientBalance(t *testing.T) {
	err := errors.Join(errorx.ErrInsufficientBalance, errors.New("extra"))
	assert.Equal(t, types.PlanErrInsufficientBalance, categorizePlanError(err))
}

func TestCategorizePlanError_UsageLimitExceeded(t *testing.T) {
	err := &component.UsageLimitExceededError{Message: "quota exceeded"}
	assert.Equal(t, types.PlanErrUsageLimitExceeded, categorizePlanError(err))
}

func TestCategorizePlanError_WrappedUsageLimit(t *testing.T) {
	inner := &component.UsageLimitExceededError{Message: "quota exceeded"}
	err := errors.Join(inner, errors.New("ctx"))
	assert.Equal(t, types.PlanErrUsageLimitExceeded, categorizePlanError(err))
}

func TestCategorizePlanError_UnknownError(t *testing.T) {
	err := errors.New("some random error")
	assert.Equal(t, types.PlanErrUnknown, categorizePlanError(err))
}

func TestCategorizePlanError_UnknownCodedError(t *testing.T) {
	err := &codedErrStub{code: "some_other_code"}
	assert.Equal(t, types.PlanErrUnknown, categorizePlanError(err))
}

func TestCategorizePlanError_WrappedCodedError(t *testing.T) {
	inner := &codedErrStub{code: "model_not_found"}
	wrapped := errors.Join(inner, errors.New("additional context"))
	assert.Equal(t, types.PlanErrModelNotFound, categorizePlanError(wrapped))
}

// --- Plan orchestration tests ---

func TestPlan_Success_Native(t *testing.T) {
	p := NewPlanner(
		&mockModelResolver{target: makeResolvedTarget("http://upstream/v1/messages", "")},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol:   string(types.ProtocolMessages),
		Task:       "messages",
		UserID:     "user1",
		Model:      "test-model",
		TenantID:   "ns-123",
		ParsedBody: &promptTextProvider{text: "hello"},
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	require.NotNil(t, plan)

	assert.NotNil(t, plan.ModelTarget)
	assert.Equal(t, "test-model", plan.ModelTarget.ModelName)
	assert.True(t, plan.BalanceOK)
	assert.True(t, plan.UsageLimitOK)
	assert.Nil(t, plan.Safety)
}

func TestPlan_Success_NoPromptText_SkipsSafety(t *testing.T) {
	p := NewPlanner(
		&mockModelResolver{target: makeResolvedTarget("http://upstream/v1/messages", "")},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{isSensitive: true, message: "blocked"},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol:   string(types.ProtocolMessages),
		Task:       "messages",
		UserID:     "user1",
		Model:      "test-model",
		TenantID:   "ns-123",
		ParsedBody: nil,
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	assert.Nil(t, plan.Safety)
}

func TestPlan_Sensitive_Flagged(t *testing.T) {
	p := NewPlanner(
		&mockModelResolver{target: makeResolvedTarget("http://upstream/v1/messages", "")},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{isSensitive: true, message: "blocked content"},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol:   string(types.ProtocolMessages),
		Task:       "messages",
		UserID:     "user1",
		Model:      "test-model",
		TenantID:   "ns-123",
		ParsedBody: &promptTextProvider{text: "sensitive text"},
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.Error(t, err)
	require.NotNil(t, plan)

	assert.Equal(t, types.PlanErrSensitive, plan.ErrorCode)
	require.NotNil(t, plan.Safety)
	assert.True(t, plan.Safety.IsSensitive)
	assert.Equal(t, "blocked content", plan.Safety.Message)
}

func TestPlan_SafetyCheckError_DoesNotBlock(t *testing.T) {
	p := NewPlanner(
		&mockModelResolver{target: makeResolvedTarget("http://upstream/v1/messages", "")},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{err: errors.New("moderation unavailable")},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol:   string(types.ProtocolMessages),
		Task:       "messages",
		UserID:     "user1",
		Model:      "test-model",
		TenantID:   "ns-123",
		ParsedBody: &promptTextProvider{text: "hello"},
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	assert.Nil(t, plan.Safety)
	assert.True(t, plan.BalanceOK)
	assert.True(t, plan.UsageLimitOK)
}

func TestPlan_ModelNotFound(t *testing.T) {
	p := NewPlanner(
		&mockModelResolver{err: &codedErrStub{code: "model_not_found"}},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolMessages),
		UserID:   "user1",
		Model:    "unknown",
		TenantID: "ns-123",
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.Error(t, err)
	assert.Equal(t, types.PlanErrModelNotFound, plan.ErrorCode)
	assert.Nil(t, plan.ModelTarget)
}

func TestPlan_ModelNilResolution(t *testing.T) {
	p := NewPlanner(
		&mockModelResolver{target: nil, err: nil},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolMessages),
		UserID:   "user1",
		Model:    "unknown",
		TenantID: "ns-123",
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.Error(t, err)
	assert.Equal(t, types.PlanErrModelNotFound, plan.ErrorCode)
}

func TestPlan_ModelUnavailable(t *testing.T) {
	p := NewPlanner(
		&mockModelResolver{err: &codedErrStub{code: "model_unavailable"}},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolMessages),
		UserID:   "user1",
		Model:    "test-model",
		TenantID: "ns-123",
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.Error(t, err)
	assert.Equal(t, types.PlanErrModelUnavailable, plan.ErrorCode)
}

func TestPlan_InsufficientBalance(t *testing.T) {
	p := NewPlanner(
		&mockModelResolver{target: makeResolvedTarget("http://upstream/v1/messages", "")},
		&mockBalanceChecker{err: errorx.ErrInsufficientBalance},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolMessages),
		UserID:   "user1",
		Model:    "test-model",
		TenantID: "ns-123",
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.Error(t, err)
	assert.Equal(t, types.PlanErrInsufficientBalance, plan.ErrorCode)
	assert.NotNil(t, plan.ModelTarget)
	assert.False(t, plan.BalanceOK)
}

func TestPlan_UsageLimitExceeded(t *testing.T) {
	p := NewPlanner(
		&mockModelResolver{target: makeResolvedTarget("http://upstream/v1/messages", "")},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{err: &component.UsageLimitExceededError{Message: "quota exceeded"}},
		&mockContentSafetyChecker{},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolMessages),
		Task:     "messages",
		UserID:   "user1",
		Model:    "test-model",
		TenantID: "ns-123",
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.Error(t, err)
	assert.Equal(t, types.PlanErrUsageLimitExceeded, plan.ErrorCode)
	assert.True(t, plan.BalanceOK)
	assert.False(t, plan.UsageLimitOK)
}

func TestPlan_Disabled_ReturnsError(t *testing.T) {
	// Chat → Responses has no adapter (AdapterNone), so routing resolves
	// to "disabled".  The Planner should return an error with PlanErrDisabled
	// and NOT check balance/quota/safety.
	target := &types.ModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
		},
		Upstream: commonType.UpstreamConfig{
			URL:      "http://upstream/v1/responses",
			Provider: "test",
		},
		Target:    "http://upstream/v1/responses",
		ModelName: "test-model",
	}

	p := NewPlanner(
		&mockModelResolver{target: target},
		&mockBalanceChecker{err: errors.New("balance should not be checked")},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolChat),
		UserID:   "user1",
		Model:    "test-model",
		TenantID: "ns-123",
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.Error(t, err)
	assert.Equal(t, types.PlanErrDisabled, plan.ErrorCode)
	// Balance check should not have been called
	assert.False(t, plan.BalanceOK)
}

func TestPlan_BackendURLFallback(t *testing.T) {
	target := makeResolvedTarget("http://upstream/v1/messages", "")
	p := NewPlanner(
		&mockModelResolver{target: target},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolMessages),
		Task:     "messages",
		UserID:   "user1",
		Model:    "test-model",
		TenantID: "ns-123",
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	assert.NotEmpty(t, plan.BackendURL)
}

func TestPlan_RoutingFieldsPopulated(t *testing.T) {
	target := makeResolvedTarget("http://upstream/v1/chat/completions", "")
	p := NewPlanner(
		&mockModelResolver{target: target},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolMessages),
		Task:     "messages",
		UserID:   "user1",
		Model:    "test-model",
		TenantID: "ns-123",
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)

	// Messages protocol with a chat upstream → adapter mode
	assert.Equal(t, string(protocol.ModeAdapter), plan.RouteMode)
	assert.NotEmpty(t, plan.AdapterKind)
	assert.NotEmpty(t, plan.UpstreamProtocol)
}

// TestPlan_NonTokenTask_SkipsUsageLimitAndSafety verifies that non-token
// tasks (e.g. "rerank", "text-to-image") skip both the usage-limit check
// and the content-safety check, since those are only meaningful for
// token-generating protocols.
func TestPlan_NonTokenTask_SkipsUsageLimitAndSafety(t *testing.T) {
	target := &types.ModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
		},
		Upstream: commonType.UpstreamConfig{
			URL:      "http://upstream/v1/rerank",
			Provider: "test",
		},
		Target:    "http://upstream/v1/rerank",
		ModelName: "test-model",
	}

	p := NewPlanner(
		&mockModelResolver{target: target},
		&mockBalanceChecker{},
		// If usage limit were checked, this would return an error.
		&mockUsageLimitChecker{err: &component.UsageLimitExceededError{Message: "should not be called"}},
		// If safety were checked, this would flag sensitive.
		&mockContentSafetyChecker{isSensitive: true, message: "should not be called"},
			nil,
	)

	meta := &types.RequestMetadata{
		Protocol:   string(types.ProtocolChat),
		Task:       "rerank",
		UserID:     "user1",
		Model:      "test-model",
		TenantID:   "ns-123",
		ParsedBody: &promptTextProvider{text: "some prompt"},
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.True(t, plan.BalanceOK)
	assert.True(t, plan.UsageLimitOK)
	assert.Nil(t, plan.Safety)
}

// --- MetricsEnricher tests ---

func TestPlan_MetricsEnricher_CalledOnSuccess(t *testing.T) {
	enricher := &stubMetricsEnricher{}
	target := makeResolvedTarget("http://upstream/v1/messages", "")
	p := NewPlanner(
		&mockModelResolver{target: target},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{},
		enricher,
	)

	meta := &types.RequestMetadata{
		Protocol:  string(types.ProtocolMessages),
		Task:      "messages",
		UserID:    "user1",
		Model:     "test-model",
		TenantID:  "ns-123",
		Streaming: true,
	}

	_, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	assert.True(t, enricher.called)
	assert.Equal(t, "test-model", enricher.modelID)
	assert.Equal(t, target, enricher.target)
	assert.True(t, enricher.isStream)
}

func TestPlan_MetricsEnricher_CalledOnResolveError(t *testing.T) {
	enricher := &stubMetricsEnricher{}
	p := NewPlanner(
		&mockModelResolver{err: &codedErrStub{code: "model_not_found"}},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&mockContentSafetyChecker{},
		enricher,
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolMessages),
		UserID:   "user1",
		Model:    "unknown",
		TenantID: "ns-123",
	}

	_, err := p.Plan(newTestGinContext(), meta)
	require.Error(t, err)
	// MetricsEnricher is called before error checking, so it runs even on
	// resolution failure.  The target is nil; the enricher falls back to
	// the requested model ID.
	assert.True(t, enricher.called)
	assert.Equal(t, "unknown", enricher.modelID)
	assert.Nil(t, enricher.target)
}
