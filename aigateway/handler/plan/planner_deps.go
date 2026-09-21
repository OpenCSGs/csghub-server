package plan

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
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

// AdmissionChecker enforces capacity admission (CapacityPolicy) for the
// resolved model target. It runs BEFORE the content-safety check (step 7 vs
// 8): the sensitive check is the expensive Plan-phase step (whitelist query
// + moderation RPC) and should only be paid by requests that have the
// capacity to run; and the safety gate's whitelist targets are built from
// the upstream provider, so admission (which may re-select the upstream)
// must settle the final target first.
//
// Boundary: the Router owns the candidate set ("where should this request
// go"); the checker only evaluates feasibility within that set ("can it be
// accepted"). When the preferred upstream is infeasible and re-selection is
// allowed (no pinned upstream, no session affinity), the checker may return
// an outcome whose ReSelectedTarget points at another candidate — it must
// never build a candidate set of its own.
type AdmissionChecker interface {
	CheckAdmission(ctx context.Context, meta *types.RequestMetadata, mt *types.ModelTarget) (*types.AdmissionOutcome, error)
}

// AdmissionReleaser is the Orchestrator's safety net: it finalizes the
// admission lease still recorded on the plan after the Execute phase returns
// (including error/panic paths). The usage commit paths finalize the same
// lease first with usage — both are idempotent.
type AdmissionReleaser interface {
	ReleaseAdmission(c *gin.Context, p *types.RequestPlan)
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
