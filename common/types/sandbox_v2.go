package types

// SandboxV2 lifecycle (create/get/delete/suspend/resume) goes through deployer.Deploy.
// The storage type is carried by the deploy table Type column (SandboxType=7 persistent,
// SandboxEphemeralType=8 ephemeral); mounts use the generic DeployExtend.VolumeMounts
// (volume_mounts jsonb column). WorkingDir is not stored and defaults to the first
// mount_path on the runner side.

// SandboxV2Type is the sandbox storage type expected by the runner.
type SandboxV2Type string

const (
	SandboxV2TypePersistent SandboxV2Type = "persistent" // long-running, suspend/resume
	SandboxV2TypeEphemeral  SandboxV2Type = "ephemeral"  // short-lived, auto-deleted when idle
)

// SandboxV2RunnerType maps the deploy table Type column to the runner's storage type string.
func SandboxV2RunnerType(deployType int) SandboxV2Type {
	if deployType == SandboxEphemeralType {
		return SandboxV2TypeEphemeral
	}
	return SandboxV2TypePersistent
}
