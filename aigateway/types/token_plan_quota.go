package types

import "fmt"

// TokenPlanQuotaExceededError reports that the tenant's active token plan
// subscription has no allowance left for the requested model in the current
// period and its overage policy rejects further use.
//
// The Planner maps this error to PlanErrQuotaExceeded (HTTP 429). Every other
// error returned by the plan-phase quota check is an infrastructure failure
// (for example the accounting service being unavailable) and must not be
// reported to the client as quota exhaustion.
type TokenPlanQuotaExceededError struct {
	ModelID           string
	UsedTokens        int64
	QuotaTokens       int64
	UsedRequestCount  int64
	QuotaRequestCount int64
}

// TokensExhausted reports whether the period's token allowance is spent.
// A non-positive allowance means the model is not token-limited.
func (e *TokenPlanQuotaExceededError) TokensExhausted() bool {
	return e.QuotaTokens > 0 && e.UsedTokens >= e.QuotaTokens
}

// RequestsExhausted reports whether the period's request allowance is spent.
// A non-positive allowance means the model is not request-limited.
func (e *TokenPlanQuotaExceededError) RequestsExhausted() bool {
	return e.QuotaRequestCount > 0 && e.UsedRequestCount >= e.QuotaRequestCount
}

func (e *TokenPlanQuotaExceededError) Error() string {
	switch {
	case e.TokensExhausted() && e.RequestsExhausted():
		return fmt.Sprintf("token plan quota exceeded for model %s: used %d of %d tokens and %d of %d requests",
			e.ModelID, e.UsedTokens, e.QuotaTokens, e.UsedRequestCount, e.QuotaRequestCount)
	case e.TokensExhausted():
		return fmt.Sprintf("token plan quota exceeded for model %s: used %d of %d tokens",
			e.ModelID, e.UsedTokens, e.QuotaTokens)
	case e.RequestsExhausted():
		return fmt.Sprintf("token plan quota exceeded for model %s: used %d of %d requests",
			e.ModelID, e.UsedRequestCount, e.QuotaRequestCount)
	default:
		// Defensive: an error constructed without an exhausted dimension
		// (e.g. a hand-built value) still reports both for context.
		return fmt.Sprintf("token plan quota exceeded for model %s: used %d of %d tokens, used %d of %d requests",
			e.ModelID, e.UsedTokens, e.QuotaTokens, e.UsedRequestCount, e.QuotaRequestCount)
	}
}
