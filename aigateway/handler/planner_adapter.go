package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

// modelResolverAdapter wraps OpenAIHandlerImpl.resolveModelTarget to satisfy
// plan.ModelResolver.  It converts the handler-private *resolvedModelTarget
// to the public *types.ModelTarget so the Planner never depends on handler
// internals.
type modelResolverAdapter struct {
	handler *OpenAIHandlerImpl
}

func (a *modelResolverAdapter) ResolveModelTarget(ctx context.Context, nsUUID, modelID string, headers http.Header, opts plan.ResolveOptions) (*types.ModelTarget, error) {
	resolved, err := a.handler.resolveModelTargetWithOptions(ctx, nsUUID, modelID, headers, modelTargetResolveOptions{
		RequiredUpstreamID: opts.RequiredUpstreamID,
	})
	if err != nil {
		return nil, err
	}
	return toTypesModelTarget(resolved), nil
}

// contentSafetyAdapter wraps the existing sensitive policy (which exposes the
// Responses-named method CheckResponsesSensitive) to satisfy
// plan.ContentSafetyChecker.  It returns a simplified (bool, string, error)
// contract so the Planner never depends on *rpc.CheckResult.
type contentSafetyAdapter struct {
	policy component.SensitivePolicy
}

func (a *contentSafetyAdapter) Check(ctx context.Context, model *types.Model, promptText, nsUUID, task string, streaming bool, provider string) (bool, string, error) {
	// Only token-generating protocols (chat/responses/messages) go through
	// the SensitivePolicy gate here.  Other tasks (text-to-image,
	// text-to-video, audio, ocr, embedding, rerank) either skip content
	// safety entirely or use CheckImagePrompts directly in their Execute
	// phase, which bypasses the SensitivePolicy gate intentionally.
	switch task {
	case "chat", "responses", "messages":
		// fall through
	default:
		return false, "", nil
	}

	shouldCheck, result, err := a.policy.CheckResponsesSensitive(ctx, model, promptText, nsUUID, streaming, provider)
	if err != nil {
		return false, "", err
	}
	if result == nil {
		return false, "", nil
	}
	return shouldCheck && result.IsSensitive, result.Reason, nil
}

// admissionCheckerAdapter adapts the OpenAI component's capacity admission
// controller to plan.AdmissionChecker. It computes the per-request inputs
// that live in the handler layer — the session-affinity key and the TPM
// pre-reservation estimate — and rebuilds the model target when admission
// re-selects a different upstream (capacity-aware fallback within the
// router-owned candidate set).
type admissionCheckerAdapter struct {
	handler *OpenAIHandlerImpl
}

func (a *admissionCheckerAdapter) CheckAdmission(ctx context.Context, meta *types.RequestMetadata, mt *types.ModelTarget) (*types.AdmissionOutcome, error) {
	if meta == nil || mt == nil || mt.Model == nil {
		return nil, nil
	}
	// Capacity-aware re-selection must never break pinned upstreams
	// (previous_response_id routing) or session affinity (KV/prefix cache
	// reuse): those requests are checked against their preferred upstream
	// only. The fallback session key mirrors the routing side
	// (resolveEndpointModelTarget) so both layers make the SAME pinned/not
	// decision for one request — the tenant ID is the shared fallback when
	// no session header is present.
	allowSelect := meta.RequiredUpstreamID == 0 && extractSessionKeyForModel(mt.Model, meta.Headers, meta.TenantID) == ""
	// Fast path: a pinned/session request (AllowSelect=false) is checked
	// against its preferred upstream ONLY, so when that upstream has no
	// enabled CapacityPolicy admission cannot apply — return "skipped"
	// without entering the checker (no candidate filtering, no estimate
	// work). Allow-select requests always consult the checker: any candidate
	// may carry a policy.
	if !allowSelect && (mt.Upstream.CapacityPolicy == nil || !mt.Upstream.CapacityPolicy.Enabled) {
		return nil, nil
	}
	// TPM estimate decision, in precedence order:
	//
	//  1. Policy-level shortcut: when NO candidate upstream enables a TPM
	//     limit (CapacityPolicy.MaxTPM <= 0 — the documented "no TPM
	//     dimension" configuration), the dimension cannot bind for any
	//     request: skip the text estimate entirely (no hot-path estimation
	//     work, no pointless reservation writes).
	//  2. Request-level guard: when a TPM limit IS configured but the
	//     request carries multimodal content, the text-based estimate would
	//     be fiction — skip the reservation for this request (image
	//     generation always reports multimodal, so it never reserves).
	//  3. Otherwise reserve the text-based estimate.
	//
	// In all cases actual usage is still committed to the TPM window at
	// finalize, so the window aggregate remains the true total usage.
	var estimatedTokens int64
	if !candidateSetHasTPMLimit(mt.Model) {
		estimatedTokens = types.AdmissionNoTPMEstimate
	} else if body, ok := meta.ParsedBody.(types.MultimodalContentProvider); ok && body.HasMultimodalContent() {
		estimatedTokens = types.AdmissionNoTPMEstimate
	} else {
		estimatedTokens = a.handler.openaiComponent.EstimateAdmissionTokens(meta.PromptText())
	}
	decision := a.handler.openaiComponent.CheckCapacityAdmission(ctx, mt.Model, mt.Upstream.ID, allowSelect, estimatedTokens)
	if decision == nil {
		return nil, nil
	}
	outcome := &types.AdmissionOutcome{Decision: decision}
	if decision.Action == types.AdmissionAdmit && decision.SelectedUpstreamID != mt.Upstream.ID {
		upstream, ok := findUpstreamConfigByID(mt.Model.Upstreams, decision.SelectedUpstreamID)
		if !ok {
			// Defensive: the selected upstream vanished from the candidate
			// set between check and rebuild. Deny instead of routing to the
			// preferred upstream with a lease held elsewhere; the acquired
			// lease self-heals via expired-lease cleanup.
			slog.WarnContext(ctx, "admission re-selected an upstream missing from the candidate set",
				slog.Int64("upstream_id", decision.SelectedUpstreamID), slog.String("model", mt.Model.ID),
				admissionTokenAttr(decision))
			denied := &types.AdmissionDecision{
				Action:            types.AdmissionReject,
				Reason:            types.AdmissionReasonCapacityExceeded,
				RetryAfterSeconds: decision.RetryAfterSeconds,
			}
			return &types.AdmissionOutcome{Decision: denied}, nil
		}
		outcome.ReSelectedTarget = a.handler.rebuildModelTargetForUpstream(mt, upstream)
	}
	return outcome, nil
}

// findUpstreamConfigByID looks up an upstream config by ID in the candidate
// set.
func findUpstreamConfigByID(upstreams []commontypes.UpstreamConfig, id int64) (commontypes.UpstreamConfig, bool) {
	for _, up := range upstreams {
		if up.ID == id {
			return up, true
		}
	}
	return commontypes.UpstreamConfig{}, false
}

// candidateSetHasTPMLimit reports whether ANY candidate upstream of the
// model enables a TPM limit. When none does, the TPM dimension cannot bind
// for any request on this model and the text-based estimate is skipped
// entirely (AdmissionNoTPMEstimate). The check must cover the whole
// candidate set because the estimate is a single value shared by all
// candidates: skipping it while one candidate has MaxTPM > 0 would silently
// disable that candidate's TPM gate too.
func candidateSetHasTPMLimit(model *types.Model) bool {
	if model == nil {
		return false
	}
	for i := range model.Upstreams {
		if policy := model.Upstreams[i].CapacityPolicy; policy != nil && policy.Enabled && policy.MaxTPM > 0 {
			return true
		}
	}
	return false
}

// rebuildModelTargetForUpstream reconstructs the resolved model target for a
// different upstream of the same model. It mirrors the endpoint-target
// construction in resolveEndpointModelTarget: endpoint/auth/provider
// overrides, model-name override, host override, and fallback attempt
// candidates excluding the new primary.
func (h *OpenAIHandlerImpl) rebuildModelTargetForUpstream(mt *types.ModelTarget, upstream commontypes.UpstreamConfig) *types.ModelTarget {
	model := mt.Model
	target := upstream.URL
	applyEndpointOverrides(model, upstream)
	return &types.ModelTarget{
		Model:          model,
		Upstream:       upstream,
		Target:         target,
		Host:           upstreamHostOverride(upstream),
		ModelName:      resolveEndpointModelName(model.ID, upstream),
		AttemptTargets: buildChatAttemptTargets(upstream, model.Upstreams, h.chatMaxFallbackAttempts()),
	}
}

// admissionReleaserAdapter implements plan.AdmissionReleaser: the
// Orchestrator's safety net that finalizes the admission lease still recorded
// on the plan after Execute returns.
type admissionReleaserAdapter struct {
	handler *OpenAIHandlerImpl
}

func (a *admissionReleaserAdapter) ReleaseAdmission(c *gin.Context, p *types.RequestPlan) {
	finalizeAdmissionLease(c.Request.Context(), a.handler, admissionLeaseFromPlan(p), nil)
}

// newPlannerDeps builds the dependency interfaces needed by plan.NewPlanner
// from an OpenAIHandlerImpl.
func newPlannerDeps(h *OpenAIHandlerImpl) (plan.ModelResolver, plan.BalanceChecker, plan.UsageLimitChecker, plan.ContentSafetyChecker, plan.AdmissionChecker) {
	return &modelResolverAdapter{handler: h},
		h.openaiComponent,
		h.openaiComponent,
		&contentSafetyAdapter{policy: h.sensitivePolicy},
		&admissionCheckerAdapter{handler: h}
}

// metricsEnricherAdapter implements plan.MetricsEnricher by delegating to the
// build-tag-gated SetMetricsModelTarget helper.  In CE builds the helper is a
// no-op, so this adapter is also a no-op.
type metricsEnricherAdapter struct{}

func (metricsEnricherAdapter) SetModelTarget(c *gin.Context, modelID string, target *types.ModelTarget, isStream bool) {
	var rt *resolvedModelTarget
	if target != nil {
		rt = modelTargetToResolved(target)
	}
	SetMetricsModelTarget(SetMetricsModelParams{
		C:           c,
		ModelID:     modelID,
		ModelTarget: rt,
		IsStream:    isStream,
	})
}

// newOrchestrator builds a fully-wired Orchestrator from an OpenAIHandlerImpl.
// It encapsulates planner dependency wiring (including metrics enrichment and
// capacity admission) and preflight tracing so protocol handlers don't repeat
// boilerplate.
func newOrchestrator(h *OpenAIHandlerImpl) *plan.Orchestrator {
	mr, bc, ulc, cs, ac := newPlannerDeps(h)
	return plan.NewOrchestrator(
		plan.NewPlanner(mr, bc, ulc, cs, ac, metricsEnricherAdapter{}),
		&preflightStarterAdapter{},
		&admissionReleaserAdapter{handler: h},
	)
}

// toTypesModelTarget converts the handler-package-private resolvedModelTarget
// to the types-package ModelTarget.
func toTypesModelTarget(r *resolvedModelTarget) *types.ModelTarget {
	if r == nil {
		return nil
	}
	return &types.ModelTarget{
		Model:          r.Model,
		Upstream:       r.Upstream,
		Target:         r.Target,
		Host:           r.Host,
		ModelName:      r.ModelName,
		AttemptTargets: r.AttemptTargets,
	}
}
