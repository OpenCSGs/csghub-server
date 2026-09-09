package plan

import (
	"context"
	"net/http"

	"opencsg.com/csghub-server/aigateway/types"
)

// ModelResolver resolves a model ID to a concrete upstream target.
// The handler package provides an adapter that wraps its private
// resolveModelTarget and converts to *types.ModelTarget.
type ModelResolver interface {
	ResolveModelTarget(ctx context.Context, username, modelID string, headers http.Header) (*types.ModelTarget, error)
}

// BalanceChecker verifies that the tenant has sufficient balance.
type BalanceChecker interface {
	CheckBalance(ctx context.Context, nsUUID string) error
}

// UsageLimitChecker verifies that the tenant has not exceeded usage quota.
type UsageLimitChecker interface {
	CheckUsageLimit(ctx context.Context, nsUUID string, model *types.Model, endpoint string) error
}

// ContentSafetyChecker checks prompt text for sensitive content.
// Returns (isSensitive, message, error). The handler adapter converts
// *rpc.CheckResult to this simplified contract so the Planner never
// depends on the rpc package.
type ContentSafetyChecker interface {
	Check(ctx context.Context, model *types.Model, promptText, tenantID string, streaming bool, provider string) (bool, string, error)
}

// CodedError is implemented by errors that carry a structured error code.
// The handler package's modelTargetError implements this method so the
// Planner can categorize errors via errors.As without depending on the
// concrete private type.
type CodedError interface {
	ModelErrorCode() string
}
