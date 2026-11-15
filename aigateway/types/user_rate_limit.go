package types

import "fmt"

// User rate limit scope and dimension values carried by
// UserRateLimitExceededError.
const (
	// UserRateLimitScopeGlobal marks the gateway-global per-user limit.
	UserRateLimitScopeGlobal = "global"
	// UserRateLimitScopeModel marks a per-model per-user limit.
	UserRateLimitScopeModel = "model"
	// UserRateLimitDimensionRPM is the requests-per-window dimension.
	UserRateLimitDimensionRPM = "rpm"
	// UserRateLimitDimensionTPM is the tokens-per-window dimension.
	UserRateLimitDimensionTPM = "tpm"
)

// UserRateLimitExceededError reports that one user exceeded the per-user
// rate limit (RPM and/or TPM) configured at the model or gateway-global
// level over the current fixed window.
//
// The Planner maps this error to PlanErrUserRateLimitExceeded (HTTP 429 with
// a Retry-After hint). Every other error returned by the plan-phase rate
// limit check is an infrastructure failure and must fail open (be swallowed
// by the limiter itself), never reported to the client as rate limiting.
type UserRateLimitExceededError struct {
	// Scope names the policy level that rejected the request:
	// UserRateLimitScopeGlobal or UserRateLimitScopeModel.
	Scope string
	// Dimension names the exhausted limit: "rpm" or "tpm".
	Dimension string
	// ModelID is the public model ID the request was planned for (empty for
	// the global scope).
	ModelID string
	// RetryAfterSeconds hints when the current window rolls over.
	RetryAfterSeconds int64
}

func (e *UserRateLimitExceededError) Error() string {
	scope := "model"
	if e.Scope == UserRateLimitScopeGlobal {
		scope = "gateway global"
	}
	switch e.Dimension {
	case UserRateLimitDimensionTPM:
		return fmt.Sprintf("user TPM rate limit exceeded on %s scope: please retry after %d seconds", scope, e.RetryAfterSeconds)
	default:
		return fmt.Sprintf("user RPM rate limit exceeded on %s scope: please retry after %d seconds", scope, e.RetryAfterSeconds)
	}
}

// ErrorCode returns the wire error code for this rejection: the scope is
// part of the code so clients can distinguish the gateway-global fallback
// from a model's own limit by the code alone.
func (e *UserRateLimitExceededError) ErrorCode() string {
	if e.Scope == UserRateLimitScopeGlobal {
		return ErrCodeUserGlobalRateLimitExceeded
	}
	return ErrCodeUserRateLimitExceeded
}
