//go:build !saas

package handler

import (
	"opencsg.com/csghub-server/aigateway/handler/plan"
)

// newTokenPlanQuotaChecker returns no checker outside the saas edition: token
// plan subscriptions only exist there, so the Planner skips the check.
func newTokenPlanQuotaChecker() plan.TokenPlanQuotaChecker {
	return nil
}
