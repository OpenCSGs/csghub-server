package admission

import (
	"context"

	"opencsg.com/csghub-server/aigateway/types"
)

// UserRateLimitComponent is the handler-facing subset of the per-user rate
// limiter (RPM/TPM configured at the model or gateway-global LLM-config
// level). The feature is EE/SaaS only: the interface stays shared so the
// planner adapter can type-assert it, while the implementation lives in
// user_rate_limit_ee.go and the CE build provides no implementation — the
// type assertion then fails and the planner skips the check entirely.
type UserRateLimitComponent interface {
	// CheckUserRateLimit enforces the effective per-user policy for the
	// planned request and CONSUMES one unit of the user's RPM budget when
	// allowed (atomic check+accounting, hard limit under concurrency). It
	// runs before capacity admission; the consumption is final regardless
	// of the later admission outcome (deliberate product trade-off — see
	// the admission package documentation). It returns
	// *types.UserRateLimitExceededError when the user is over the limit;
	// every other condition (no policy, no tenant, Redis failure) allows
	// the request.
	CheckUserRateLimit(ctx context.Context, meta *types.RequestMetadata, mt *types.ModelTarget) error
}
