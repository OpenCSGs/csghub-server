//go:build !ee && !saas

package handler

import (
	"opencsg.com/csghub-server/aigateway/handler/plan"
)

// CE has no capacity admission: the planner wires a nil AdmissionChecker
// (admission is then skipped) and the Orchestrator wires no lease-release
// safety net. The OpenAIComponent admission methods are CE no-ops too, so
// the execution-path helpers degrade to "proceeds unprotected".
func newAdmissionChecker(_ *OpenAIHandlerImpl) plan.AdmissionChecker {
	return nil
}

// newAdmissionReleaser returns nil on CE: with a nil checker no lease can
// exist, so there is nothing for the safety net to release.
func newAdmissionReleaser(_ *OpenAIHandlerImpl) plan.AdmissionReleaser {
	return nil
}
