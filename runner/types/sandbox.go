package types

import (
	"time"

	"opencsg.com/csghub-server/common/types"
)

// SandboxSpec defines sandbox specification
type SandboxRequest struct {
	SandboxName string                `json:"sandbox_name"`
	Image       string                `json:"image,omitempty"`
	Hardware    types.HardWare        `json:"hardware,omitempty"`
	ClusterID   string                `json:"cluster_id"`
	DeployID    string                `json:"deploy_id"`
	TaskId      string                `json:"task_id"`
	UserUUID    string                `json:"user_id"`
	ResourceID  int64                 `json:"resource_id"`
	EnvVars     map[string]string     `json:"env_vars,omitempty"`
	Labels      map[string]string     `json:"labels,omitempty"`
	TemplateID  string                `json:"templateID,omitempty"`
	Timeout     int                   `json:"timeout,omitempty"`
	Volumes     []types.SandboxVolume `json:"volumes,omitempty"`
	Port        int                   `json:"port,omitempty"`
	DeployName  string                `json:"deploy_name,omitempty"`
}

type SandboxResponse struct {
	ID      int64  `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type SandboxEvent struct {
	Status       int    `json:"status"`
	SandboxID    string `json:"sandbox_id"`
	DeployId     string `json:"deploy_id"`
	DeployTaskId string `json:"deploy_task_id"`
	Message      string `json:"message"` // event message
	Reason       string `json:"reason"`  // event reason
}

type SandboxDeleteRequest struct {
	SandboxID string `json:"sandbox_id"`
	ClusterID string `json:"cluster_id"`
}

type Sandbox struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

type SandboxStatus struct {
	Status  int
	Message string
	Reason  string
}
