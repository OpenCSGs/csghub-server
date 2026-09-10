package plan

import (
	"context"
	"net/http"

	"opencsg.com/csghub-server/aigateway/types"
)

// ModelResolver resolves a model ID to a concrete upstream target.
// The handler package provides an adapter that wraps its private
// resolveModelTarget and converts to *types.ModelTarget.
//
// ResolveOptions carries optional parameters that some protocols need
// during model resolution (e.g. Responses passes RequiredUpstreamID so
// that previous_response_id routing pins the request to the original
// upstream).  Most protocols leave it zero-valued.
type ModelResolver interface {
	ResolveModelTarget(ctx context.Context, nsUUID, modelID string, headers http.Header, opts ResolveOptions) (*types.ModelTarget, error)
}

// ResolveOptions passes protocol-specific model resolution hints from
// the Extract phase through the Planner to the ModelResolver adapter.
type ResolveOptions struct {
	// RequiredUpstreamID forces the resolver to select the upstream with
	// this ID.  Used by the Responses protocol when the client supplies a
	// previous_response_id whose original upstream must be reused.
	RequiredUpstreamID int64
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
//
// The task parameter lets the adapter dispatch to the correct safety
// policy path (e.g. "chat"/"responses"/"messages" go through the
// SensitivePolicy gate, while "text-to-image"/"text-to-video" etc.
// are skipped here because they use a separate CheckImagePrompts path
// that bypasses the gate — those remain in each handler's Execute).
type ContentSafetyChecker interface {
	Check(ctx context.Context, model *types.Model, promptText, nsUUID, task string, streaming bool, provider string) (bool, string, error)
}

// CodedError is implemented by errors that carry a structured error code.
// The handler package's modelTargetError implements this method so the
// Planner can categorize errors via errors.As without depending on the
// concrete private type.
type CodedError interface {
	ModelErrorCode() string
}
