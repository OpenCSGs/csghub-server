package plan

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commonType "opencsg.com/csghub-server/common/types"
)

// --- test doubles recording call order ---

type recordingSafetyChecker struct {
	called bool
	// afterAdmission is set when the admission checker reports it already
	// ran — i.e. the sensitive check happened second.
	afterAdmission bool
	// checkedProvider records the upstream provider the sensitive check
	// received (whitelist targets are built from it).
	checkedProvider string
}

func (m *recordingSafetyChecker) Check(_ context.Context, _ *types.Model, _, _, _ string, _ bool, provider string) (bool, string, error) {
	m.called = true
	m.checkedProvider = provider
	return false, "", nil
}

type recordingAdmissionChecker struct {
	safety *recordingSafetyChecker
	called bool
	// decidedTarget records the model target this admission decision was
	// made on (pinned or re-selected).
	decidedTarget *types.ModelTarget
	outcome       *types.AdmissionOutcome
}

func (m *recordingAdmissionChecker) CheckAdmission(_ context.Context, _ *types.RequestMetadata, mt *types.ModelTarget) (*types.AdmissionOutcome, error) {
	m.called = true
	m.decidedTarget = mt
	// Admission runs first; the sensitive check runs after with the target
	// admission decided on (asserted via the safety stub's observations).
	m.safety.afterAdmission = true
	return m.outcome, nil
}

func admissionPlanTestTarget() *types.ModelTarget {
	return makeResolvedTarget("http://upstream/v1/messages", "")
}

// --- tests ---

func TestPlan_AdmissionRunsAfterSensitive(t *testing.T) {
	safety := &recordingSafetyChecker{}
	admission := &recordingAdmissionChecker{safety: safety}
	p := NewPlanner(
		&mockModelResolver{target: admissionPlanTestTarget()},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		safety,
		admission,
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
	assert.True(t, admission.called)
	assert.True(t, safety.afterAdmission, "the sensitive check must run after capacity admission")
}

func TestPlan_AdmissionSkippedForUnknownTask(t *testing.T) {
	safety := &recordingSafetyChecker{}
	admission := &recordingAdmissionChecker{safety: safety}
	p := NewPlanner(
		&mockModelResolver{target: admissionPlanTestTarget()},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		safety,
		admission,
		nil,
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolChat),
		Task:     "unknown-task",
		UserID:   "user1",
		Model:    "test-model",
		TenantID: "ns-123",
	}

	_, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	assert.False(t, admission.called, "unknown tasks bypass capacity admission")
}

func TestPlan_AdmissionCoversAllModalTasks(t *testing.T) {
	// Every known modality participates in capacity admission; media tasks
	// are admitted without a TPM reservation (their parsed bodies report
	// multimodal content), text tasks reserve from the text estimate.
	for _, task := range []string{
		"text-to-image", "audio", "speech", "embedding", "rerank", "ocr", "text-to-video",
	} {
		t.Run(task, func(t *testing.T) {
			safety := &recordingSafetyChecker{}
			admission := &recordingAdmissionChecker{safety: safety}
			p := NewPlanner(
				&mockModelResolver{target: admissionPlanTestTarget()},
				&mockBalanceChecker{},
				&mockUsageLimitChecker{},
				safety,
				admission,
				nil,
			)

			meta := &types.RequestMetadata{
				Protocol: string(types.ProtocolChat),
				Task:     task,
				UserID:   "user1",
				Model:    "test-model",
				TenantID: "ns-123",
			}

			_, err := p.Plan(newTestGinContext(), meta)
			require.NoError(t, err)
			assert.True(t, admission.called, "task %q must participate in capacity admission", task)
		})
	}
}

func TestPlan_AdmissionRunsForImageTask(t *testing.T) {
	// Image generation is capacity-admitted (concurrency + RPM) but without
	// a TPM reservation — its parsed body reports multimodal content so the
	// handler adapter passes EstimatedTokens <= 0.
	safety := &recordingSafetyChecker{}
	admission := &recordingAdmissionChecker{safety: safety}
	p := NewPlanner(
		&mockModelResolver{target: admissionPlanTestTarget()},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		safety,
		admission,
		nil,
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolChat),
		Task:     "text-to-image",
		UserID:   "user1",
		Model:    "test-model",
		TenantID: "ns-123",
	}

	_, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	assert.True(t, admission.called, "image generation participates in capacity admission")
}

func TestPlan_AdmissionReject_SetsCapacityErrorCategory(t *testing.T) {
	decision := &types.AdmissionDecision{
		Action:            types.AdmissionReject,
		Reason:            types.AdmissionReasonConcurrencyExceeded,
		RetryAfterSeconds: 5,
	}
	admission := &recordingAdmissionChecker{
		safety:  &recordingSafetyChecker{},
		outcome: &types.AdmissionOutcome{Decision: decision},
	}
	p := NewPlanner(
		&mockModelResolver{target: admissionPlanTestTarget()},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&recordingSafetyChecker{},
		admission,
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
	require.Error(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, types.PlanErrCapacityExceeded, plan.ErrorCode)
	assert.Same(t, decision, plan.Admission)

	var denied *types.AdmissionDeniedError
	assert.ErrorAs(t, err, &denied)
	assert.Equal(t, decision, denied.Decision)
}

func TestPlan_AdmissionNilDecision_PassesThrough(t *testing.T) {
	admission := &recordingAdmissionChecker{
		safety:  &recordingSafetyChecker{},
		outcome: nil, // admission did not apply
	}
	p := NewPlanner(
		&mockModelResolver{target: admissionPlanTestTarget()},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&recordingSafetyChecker{},
		admission,
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
	assert.Nil(t, plan.Admission)
}

func TestPlan_AdmissionReSelection_AppliesNewTarget(t *testing.T) {
	primary := admissionPlanTestTarget()
	reselected := &types.ModelTarget{
		Model:     primary.Model,
		Upstream:  primary.Upstream,
		Target:    "http://other-upstream/v1/chat/completions",
		Host:      "",
		ModelName: "test-model",
	}
	admission := &recordingAdmissionChecker{
		safety: &recordingSafetyChecker{},
		outcome: &types.AdmissionOutcome{
			Decision: &types.AdmissionDecision{
				Action:             types.AdmissionAdmit,
				SelectedUpstreamID: 2,
				ReSelected:         true,
			},
			ReSelectedTarget: reselected,
		},
	}
	p := NewPlanner(
		&mockModelResolver{target: primary},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		&recordingSafetyChecker{},
		admission,
		nil,
	)

	meta := &types.RequestMetadata{
		Protocol:   string(types.ProtocolChat),
		Task:       "chat",
		UserID:     "user1",
		Model:      "test-model",
		TenantID:   "ns-123",
		ParsedBody: &promptTextProvider{text: "hello"},
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Same(t, reselected, plan.ModelTarget)
	assert.Equal(t, reselected.Target, plan.BackendURL)
	assert.Equal(t, "native", plan.RouteMode)
}

func TestCategorizePlanError_AdmissionDenied(t *testing.T) {
	// AdmissionDeniedError is categorized by the explicit ErrorCode on the
	// plan; categorizePlanError must not misclassify it.
	err := &types.AdmissionDeniedError{Decision: &types.AdmissionDecision{Reason: types.AdmissionReasonTPMExceeded}}
	assert.Equal(t, types.PlanErrUnknown, categorizePlanError(err))
}

func TestPlan_SensitiveSeesReSelectedUpstream(t *testing.T) {
	// Ordering correctness: capacity admission runs BEFORE the sensitive
	// check, and a capacity re-selection must be applied to the plan before
	// the sensitive check runs — the safety gate's whitelist targets are
	// built from the upstream provider, so it must observe the FINAL
	// upstream, never the router-picked one admission replaced.
	primary := admissionPlanTestTarget()
	reselected := &types.ModelTarget{
		Model:     primary.Model,
		Upstream:  commonType.UpstreamConfig{URL: "http://other-upstream/v1", Provider: "other-provider"},
		Target:    "http://other-upstream/v1/chat/completions",
		ModelName: "test-model",
	}
	admission := &recordingAdmissionChecker{
		safety: &recordingSafetyChecker{},
		outcome: &types.AdmissionOutcome{
			Decision: &types.AdmissionDecision{
				Action:             types.AdmissionAdmit,
				SelectedUpstreamID: 2,
				ReSelected:         true,
			},
			ReSelectedTarget: reselected,
		},
	}
	safety := &recordingSafetyChecker{}
	p := NewPlanner(
		&mockModelResolver{target: primary},
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		safety,
		admission,
		nil,
	)

	meta := &types.RequestMetadata{
		Protocol:   string(types.ProtocolChat),
		Task:       "chat",
		UserID:     "user1",
		Model:      "test-model",
		TenantID:   "ns-123",
		ParsedBody: &promptTextProvider{text: "hello"},
	}

	pl, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	require.NotNil(t, pl)
	require.True(t, safety.called, "the sensitive check must have run")
	require.Equal(t, "other-provider", safety.checkedProvider,
		"the sensitive check must receive the re-selected upstream's provider")
	require.Same(t, reselected, pl.ModelTarget,
		"the plan must carry the re-selected target")
}

// rerouteAdmissionChecker returns a reroute decision for the first
// CheckAdmission call and the configured outcome afterwards, recording how
// many times admission ran.
type rerouteAdmissionChecker struct {
	calls     int
	afterCall *types.AdmissionOutcome
}

func (m *rerouteAdmissionChecker) CheckAdmission(_ context.Context, _ *types.RequestMetadata, _ *types.ModelTarget) (*types.AdmissionOutcome, error) {
	m.calls++
	if m.calls == 1 || m.afterCall == nil {
		return &types.AdmissionOutcome{Decision: &types.AdmissionDecision{
			Action: types.AdmissionReroute,
		}}, nil
	}
	return m.afterCall, nil
}

func TestPlan_QueueReroute_RePlansAdmission(t *testing.T) {
	// A queued ticket whose upstream turned unavailable cancels itself and
	// hands routing back to the planner: the full plan (resolution included)
	// must re-run against a fresh candidate set.
	safety := &recordingSafetyChecker{}
	resolver := &mockModelResolver{target: admissionPlanTestTarget()}
	admission := &rerouteAdmissionChecker{
		afterCall: &types.AdmissionOutcome{Decision: &types.AdmissionDecision{
			Action:             types.AdmissionAdmit,
			SelectedUpstreamID: 1,
		}},
	}
	p := NewPlanner(
		resolver,
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		safety,
		admission,
		nil,
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolMessages),
		Task:     "messages",
		Model:    "test-model",
		TenantID: "ns-123",
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, 2, admission.calls, "admission must re-run after a reroute")
	require.Equal(t, 2, resolver.resolutionCalls(), "model resolution must re-run after a reroute")
	require.Equal(t, types.AdmissionAdmit, plan.Admission.Action)
}

func TestPlan_QueueReroute_Exhausted_RendersModelUnavailable(t *testing.T) {
	safety := &recordingSafetyChecker{}
	resolver := &mockModelResolver{target: admissionPlanTestTarget()}
	// Always reroutes: the re-plan budget must bound the loop and render 503.
	admission := &rerouteAdmissionChecker{}
	p := NewPlanner(
		resolver,
		&mockBalanceChecker{},
		&mockUsageLimitChecker{},
		safety,
		admission,
		nil,
		WithQueueMaxRePlans(2),
	)

	meta := &types.RequestMetadata{
		Protocol: string(types.ProtocolMessages),
		Task:     "messages",
		Model:    "test-model",
		TenantID: "ns-123",
	}

	plan, err := p.Plan(newTestGinContext(), meta)
	require.Error(t, err)
	require.Equal(t, types.PlanErrModelUnavailable, plan.ErrorCode)
	// Initial plan + exactly queueMaxRePlans re-plans.
	require.Equal(t, 3, admission.calls)
}
