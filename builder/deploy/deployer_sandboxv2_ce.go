//go:build !ee && !saas

package deploy

// SandboxV2Deployer is empty in CE builds: sandbox-v2 is ee/saas only.
type SandboxV2Deployer interface {
}
