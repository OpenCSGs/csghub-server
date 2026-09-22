//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

// CE stubs for the capacity admission methods on OpenAIComponent. Capacity
// admission (Concurrency/RPM/TPM enforcement backed by Redis) is an EE/SaaS
// capability; on CE every method degrades to "admission does not apply":
// Check/Acquire return nil (the caller admits directly, zero Redis calls)
// and the finalize paths are no-ops. The plan phase skips admission entirely
// because newPlannerDeps wires a nil AdmissionChecker on CE.

func (m *openaiComponentImpl) CheckCapacityAdmission(_ context.Context, _ *types.Model, _ int64, _ bool, _ int64) *types.AdmissionDecision {
	return nil
}

func (m *openaiComponentImpl) AcquireCapacityAdmission(_ context.Context, _ *types.Model, _ int64, _ int64) *types.AdmissionDecision {
	return nil
}

func (m *openaiComponentImpl) FinalizeCapacityAdmission(_ context.Context, _ *types.AdmissionLease, _ *token.Usage) {
}

func (m *openaiComponentImpl) FinalizeCanceledCapacityAdmission(_ context.Context, _ *types.AdmissionLease, _ *token.Usage) {
}

func (m *openaiComponentImpl) EstimateAdmissionTokens(_ string) int64 {
	return 0
}
