package types

// SandboxV2 lifecycle (create/get/delete/suspend/resume) goes through deployer.Deploy.
// The storage type is carried by the deploy table Type column (SandboxType=7 persistent,
// SandboxEphemeralType=8 ephemeral); mounts use the generic DeployExtend.VolumeMounts
// (volume_mounts jsonb column). WorkingDir is not stored and defaults to the first
// mount_path on the runner side.

// SandboxV2RuntimeFramework is the runtime_framework value set on deploy records
// created through the deployer for sandbox V2. The aigateway reads this field to
// route proxy requests to the V2 runner endpoint instead of the V1 one.
const SandboxV2RuntimeFramework = "sandbox-v2"

// SandboxV2CreateRequest is the component-layer create request for a sandbox-v2
// sandbox. It deliberately does NOT carry resource-derived fields (Hardware,
// ClusterID, SKU): those are resolved inside SandboxV2Component.Create from
// ResourceID, or auto-allocated from a free sandbox resource when ResourceID is 0.
type SandboxV2CreateRequest struct {
	DeployName     string                 `json:"deploy_name,omitempty"`
	SvcName        string                 `json:"svc_name,omitempty"`
	ImageID        string                 `json:"image_id,omitempty"`
	Env            string                 `json:"env,omitempty"` // JSON-encoded map[string]string
	ContainerPort  int                    `json:"container_port,omitempty"`
	UserUUID       string                 `json:"user_uuid,omitempty"`
	ResourceID     int64                  `json:"resource_id,omitempty"`     // 0 = auto-allocate a free sandbox resource
	MinCPU         string                 `json:"min_cpu,omitempty"`         // lower bound for auto-allocation, e.g. "500m"
	MinMemory      string                 `json:"min_memory,omitempty"`      // lower bound for auto-allocation, e.g. "512Mi"
	ReadinessProbe *SandboxReadinessProbe `json:"readiness_probe,omitempty"` // app-level readiness (persistent only)
	Timeout        int                    `json:"timeout,omitempty"`         // idle-reclaim timeout in minutes; 0 = permanent (paid resources only)
	VolumeMounts   []VolumeMount          `json:"volume_mounts,omitempty"`
	MinReplica     int                    `json:"min_replica,omitempty"`
	MaxReplica     int                    `json:"max_replica,omitempty"`
}

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
