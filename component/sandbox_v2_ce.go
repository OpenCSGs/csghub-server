//go:build !ee && !saas

package component

// SandboxV2Component is empty in CE builds: sandbox-v2 is ee/saas only.
// The CSGClawAgentInstanceAdapter that uses this interface is also ee/saas only,
// so no CE code references it.
type SandboxV2Component interface{}
