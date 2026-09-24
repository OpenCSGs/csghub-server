package plan

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

// mockAutoModelSelector records what the Planner asked for and returns a
// canned decision.
type mockAutoModelSelector struct {
	autoID        string
	decision      *types.AutoRouteDecision
	err           error
	called        bool
	gotNsUUID     string
	gotMessage    string
	gotUpstreamID int64
}

func (m *mockAutoModelSelector) AutoModelID() string { return m.autoID }

func (m *mockAutoModelSelector) ResolveAutoModel(_ context.Context, req types.AutoRouteRequest) (*types.AutoRouteDecision, error) {
	m.called = true
	m.gotNsUUID = req.TenantID
	m.gotUpstreamID = req.RequiredUpstreamID
	if len(req.Input.Messages) > 0 {
		m.gotMessage = req.Input.Messages[0].Content
	}
	return m.decision, m.err
}

// autoRouteBody is a parsed body that can describe itself to the ranking
// service, as every routable protocol's body does.
type autoRouteBody struct {
	text string
}

func (b *autoRouteBody) PromptText() string { return b.text }

func (b *autoRouteBody) AutoRouteContext() types.AutoRouteInput {
	return types.AutoRouteInput{Messages: []types.AutoRouteMessage{{Role: "user", Content: b.text}}}
}

func autoPlannerDeps(selector AutoModelSelector) PlannerDeps {
	return PlannerDeps{
		ModelResolver:     &mockModelResolver{target: makeResolvedTarget("http://upstream/v1/chat/completions", "")},
		BalanceChecker:    &mockBalanceChecker{},
		UsageLimitChecker: &mockUsageLimitChecker{},
		ContentSafety:     &mockContentSafetyChecker{},
		AutoModelSelector: selector,
	}
}

func autoRouteMeta(model string) *types.RequestMetadata {
	return &types.RequestMetadata{
		Protocol:   string(types.ProtocolChat),
		Task:       "chat",
		UserID:     "user1",
		Model:      model,
		TenantID:   "ns-123",
		ParsedBody: &autoRouteBody{text: "write a csv dedupe function"},
	}
}

func TestPlan_AutoModel_ReplacesVirtualModel(t *testing.T) {
	selector := &mockAutoModelSelector{
		autoID: "auto",
		decision: &types.AutoRouteDecision{
			ModelID:       "glm-5.1",
			BenchmarkID:   "glm-5.1",
			Rank:          2,
			Score:         0.82,
			IndexVersion:  "live-abc",
			PolicyVersion: "v2/1.0",
		},
	}
	p := NewPlanner(autoPlannerDeps(selector))

	meta := autoRouteMeta("auto")
	pl, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)

	require.True(t, selector.called)
	assert.Equal(t, "ns-123", selector.gotNsUUID)
	assert.Equal(t, "write a csv dedupe function", selector.gotMessage)

	assert.Equal(t, "glm-5.1", meta.Model, "later stages must see the concrete model, not the placeholder")
	require.NotNil(t, pl.AutoRoute)
	assert.Equal(t, "glm-5.1", pl.AutoRoute.ModelID)
	assert.Equal(t, 2, pl.AutoRoute.Rank)
	assert.Equal(t, "live-abc", pl.AutoRoute.IndexVersion)
	require.NotNil(t, pl.ModelTarget)
}

// The virtual ID is matched exactly, like every other model ID, so that
// the listing, the planner and the catalogue lookup cannot disagree about
// which requests are for the virtual model.
func TestPlan_AutoModel_MatchesVirtualModelIDExactly(t *testing.T) {
	for _, requested := range []string{"Auto", "AUTO", "auto-routing"} {
		t.Run(requested, func(t *testing.T) {
			selector := &mockAutoModelSelector{autoID: "auto", decision: &types.AutoRouteDecision{ModelID: "glm-5.1"}}
			p := NewPlanner(autoPlannerDeps(selector))

			meta := autoRouteMeta(requested)
			_, err := p.Plan(newTestGinContext(), meta)
			require.NoError(t, err)
			assert.False(t, selector.called, "only the exact ID selects automatic routing")
			assert.Equal(t, requested, meta.Model)
		})
	}

	// Surrounding whitespace is still tolerated.
	selector := &mockAutoModelSelector{autoID: "auto", decision: &types.AutoRouteDecision{ModelID: "glm-5.1"}}
	p := NewPlanner(autoPlannerDeps(selector))
	_, err := p.Plan(newTestGinContext(), autoRouteMeta("  auto "))
	require.NoError(t, err)
	assert.True(t, selector.called)
}

// A real model owning the virtual ID makes the request an ordinary one:
// the planner adopts that model's own ID and records no routing decision.
func TestPlan_AutoModel_ShadowedByARealModel(t *testing.T) {
	selector := &mockAutoModelSelector{
		autoID:   "auto",
		decision: &types.AutoRouteDecision{ModelID: "auto", Shadowed: true},
	}
	p := NewPlanner(autoPlannerDeps(selector))

	meta := autoRouteMeta("auto")
	pl, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	assert.Equal(t, "auto", meta.Model)
	assert.Nil(t, pl.AutoRoute, "a shadowed request is not a routing decision")
}

// A conversation pinned to an upstream must carry that pin into
// selection, or the model chosen will not own the upstream that
// resolution then insists on.
func TestPlan_AutoModel_CarriesTheUpstreamPin(t *testing.T) {
	selector := &mockAutoModelSelector{
		autoID:   "auto",
		decision: &types.AutoRouteDecision{ModelID: "glm-5.1", Pinned: true},
	}
	p := NewPlanner(autoPlannerDeps(selector))

	meta := autoRouteMeta("auto")
	meta.RequiredUpstreamID = 42
	pl, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	assert.Equal(t, int64(42), selector.gotUpstreamID, "the pin has to reach the selector")
	assert.Equal(t, "glm-5.1", meta.Model)
	require.NotNil(t, pl.AutoRoute)
	assert.True(t, pl.AutoRoute.Pinned)
}

func TestPlan_AutoModel_IgnoresOtherModelIDs(t *testing.T) {
	selector := &mockAutoModelSelector{autoID: "auto"}
	p := NewPlanner(autoPlannerDeps(selector))

	meta := autoRouteMeta("glm-5.1")
	_, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	assert.False(t, selector.called)
	assert.Equal(t, "glm-5.1", meta.Model)
}

func TestPlan_AutoModel_SkippedWhenRoutingIsNotConfigured(t *testing.T) {
	// An empty AutoModelID means the feature is off, so "auto" is just an
	// ordinary model name and is resolved like any other.
	selector := &mockAutoModelSelector{autoID: ""}
	p := NewPlanner(autoPlannerDeps(selector))

	meta := autoRouteMeta("auto")
	_, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	assert.False(t, selector.called)
	assert.Equal(t, "auto", meta.Model)
}

// A ranking service that is down is a temporary condition: it has to
// surface as a retryable unavailability, not as a model that does not
// exist, or clients and agent frameworks will stop retrying.
func TestPlan_AutoModel_ServiceDownIsRetryable(t *testing.T) {
	selector := &mockAutoModelSelector{autoID: "auto", err: &codedErrStub{code: "model_unavailable"}}
	p := NewPlanner(autoPlannerDeps(selector))

	pl, err := p.Plan(newTestGinContext(), autoRouteMeta("auto"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "automatic model routing failed")
	assert.Equal(t, types.PlanErrModelUnavailable, pl.ErrorCode)
	assert.Nil(t, pl.ModelTarget, "no upstream is resolved once selection fails")
}

// Nothing matching the caller's catalogue will not fix itself on retry,
// so that stays a not-found.
func TestPlan_AutoModel_NoUsableCandidateIsNotFound(t *testing.T) {
	selector := &mockAutoModelSelector{autoID: "auto", err: &codedErrStub{code: "model_not_found"}}
	p := NewPlanner(autoPlannerDeps(selector))

	pl, err := p.Plan(newTestGinContext(), autoRouteMeta("auto"))
	require.Error(t, err)
	assert.Equal(t, types.PlanErrModelNotFound, pl.ErrorCode)
}

// A plan error only renders an HTTP response, so the rejection has to be
// logged here or the 404 is untraceable.
func TestPlan_AutoModel_LogsTheRejection(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	selector := &mockAutoModelSelector{autoID: "auto", err: errors.New("ranking unavailable")}
	p := NewPlanner(autoPlannerDeps(selector))

	_, err := p.Plan(newTestGinContext(), autoRouteMeta("auto"))
	require.Error(t, err)

	out := buf.String()
	assert.Contains(t, out, "automatic model routing failed")
	assert.Contains(t, out, "ranking unavailable")
	assert.Contains(t, out, "requested_model=auto")
	assert.Contains(t, out, "protocol=chat")
	assert.Contains(t, out, "tenant=ns-123")
}

func TestPlan_AutoModel_NoSelectorConfigured(t *testing.T) {
	deps := autoPlannerDeps(nil)
	deps.AutoModelSelector = nil
	p := NewPlanner(deps)

	meta := autoRouteMeta("auto")
	_, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	assert.Equal(t, "auto", meta.Model)
}
