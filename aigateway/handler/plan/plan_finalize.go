package plan

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
)

// finalizePlan runs the post-Plan housekeeping shared by every Plan outcome
// (admitted, rejected, timed out, disconnected): it records the queue wait
// accumulated by the Planner onto the request metrics, and — when an
// admission releaser is configured and the Planner produced a plan —
// returns the admission-lease release safety net for the caller to defer
// (nil when there is nothing to release).
//
// The returned closure reads the lease from the plan at DEFER EXECUTION
// time, not registration time: a lease acquired later (e.g. the
// availability fallback re-acquiring a pinned lease in the attempt loop
// after a fail-open Plan) must be released too, otherwise the renewer would
// keep it alive forever. Execute blocks until the response is fully
// written, so the deferred release runs after the upstream attempt
// finished — covering success, execute errors and panics. The release is
// ownership-guarded and idempotent: when the protocol's async usage commit
// already finalized the lease (with real usage), this is a Redis no-op.
func (o *Orchestrator) finalizePlan(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) func() {
	recordQueueWait(c, meta)
	if o.admissionReleaser == nil || p == nil {
		return nil
	}
	return func() { o.admissionReleaser.ReleaseAdmission(c, p) }
}
