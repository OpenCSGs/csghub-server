package component

import (
	"context"

	"opencsg.com/csghub-server/aigateway/component/admission"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

// openaiComponentImpl glue for the capacity admission controller (the
// controller itself lives in the admission package).

// CheckCapacityAdmission evaluates CapacityPolicy admission for the resolved
// model target (initial admission: selection + occupation).
// estimatedTokens comes from EstimateAdmissionTokens on the plan path;
// traceID is the gateway request ID for log correlation.
func (m *openaiComponentImpl) CheckCapacityAdmission(ctx context.Context, model *types.Model, preferredUpstreamID int64, allowSelect bool, estimatedTokens int64) *types.AdmissionDecision {
	return m.getCapacityAdmission().Check(ctx, types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: preferredUpstreamID,
		AllowSelect:         allowSelect,
		EstimatedTokens:     estimatedTokens,
	})
}

// AcquireCapacityAdmission pins a specific upstream and atomically
// check+acquires its lease (fallback operation: no re-selection). The
// estimatedTokens are reused from the original admission's lease.
func (m *openaiComponentImpl) AcquireCapacityAdmission(ctx context.Context, model *types.Model, upstreamID int64, estimatedTokens int64) *types.AdmissionDecision {
	return m.getCapacityAdmission().Acquire(ctx, model, upstreamID, estimatedTokens)
}

// FinalizeCapacityAdmission releases the lease and commits usage. It is
// idempotent: a second call (e.g. the Orchestrator's safety-net defer after
// the async usage commit already finalized) is a Redis no-op.
func (m *openaiComponentImpl) FinalizeCapacityAdmission(ctx context.Context, lease *types.AdmissionLease, usage *token.Usage) {
	m.getCapacityAdmission().Finalize(ctx, lease, usage)
}

// FinalizeCanceledCapacityAdmission finalizes a client-canceled attempt and
// releases its RPM window entry (see the interface doc).
func (m *openaiComponentImpl) FinalizeCanceledCapacityAdmission(ctx context.Context, lease *types.AdmissionLease, usage *token.Usage) {
	m.getCapacityAdmission().FinalizeCanceled(ctx, lease, usage)
}

func (m *openaiComponentImpl) EstimateAdmissionTokens(promptText string) int64 {
	return m.getCapacityAdmission().EstimateAdmissionTokens(promptText)
}

func (m *openaiComponentImpl) getCapacityAdmission() admission.CapacityAdmissionController {
	if m.capacityAdmission != nil {
		// Injected directly (tests): use it as-is.
		return m.capacityAdmission
	}
	// Memoize: the controller owns the per-pod lease renewer goroutine and
	// must be a singleton — building it per call would leak one renewer
	// goroutine per admitted request (Finalize would unregister on a
	// different instance).
	m.capacityAdmissionOnce.Do(func() {
		m.capacityAdmission = admission.NewCapacityAdmissionController(m.modelListCache, m.capacityAdmissionOptions)
	})
	return m.capacityAdmission
}
