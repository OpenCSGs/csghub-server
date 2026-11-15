//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

// Per-user rate limiting (RPM/TPM) is an EE/SaaS feature: the CE build
// carries no limiter implementation and no policy wiring, so the planner's
// type assertion on admission.UserRateLimitComponent fails and the check is
// skipped. This no-op keeps the shared RecordUsageFromTokenUsage
// post-Execute hook compiling; it must never enforce anything.
func (m *openaiComponentImpl) CommitUserRateLimitUsage(_ context.Context, _ string, _ *types.Model, _ *token.Usage) {
}
