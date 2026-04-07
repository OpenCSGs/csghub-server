package types

import (
	"time"
)

type SandboxErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type SandboxVolume struct {
	SandboxMountSubpath string `json:"sandbox_mount_subpath"`
	SandboxMountPath    string `json:"sandbox_mount_path"`
	ReadOnly            bool   `json:"read_only"`
}

type SandboxCreateRequest struct {
	UUID         string            `json:"-"`
	Image        string            `json:"image" binding:"required"`
	ResourceID   int64             `json:"resource_id" binding:"required"`
	SandboxName  string            `json:"sandbox_name" binding:"required"`
	Environments map[string]string `json:"environments,omitempty"`
	Volumes      []SandboxVolume   `json:"volumes,omitempty"`
	Port         int               `json:"port,omitempty"`
	Timeout      int               `json:"timeout,omitempty"`
}

type SandboxUpdateRequest struct {
	UUID         string            `json:"-"`
	Image        string            `json:"image" binding:"required"`
	ResourceID   int64             `json:"resource_id" binding:"required"`
	Environments map[string]string `json:"environments,omitempty"`
	Volumes      []SandboxVolume   `json:"volumes,omitempty"`
	Port         int               `json:"port,omitempty"`
	Timeout      int               `json:"timeout,omitempty"`
}

type SandboxCreateResponse struct {
	SandboxName  string            `json:"sandbox_name"`
	Image        string            `json:"image"`
	Environments map[string]string `json:"environments"`
	Volumes      []SandboxVolume   `json:"volumes"`
}

type SandboxState struct {
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	StartedAt time.Time `json:"started_at,omitempty"`
}

type SandboxStateEvent struct {
	SandboxName string `json:"sandbox_name"`
	Message     string `json:"message"`
	Status      string `json:"status"`
}

type SandboxResponse struct {
	Spec  SandboxCreateResponse `json:"spec"`
	State SandboxState          `json:"state"`
}

type SandboxEvent struct {
	Status       int    `json:"status"`
	SandboxID    string `json:"sandbox_id"`
	DeployId     string `json:"deploy_id"`
	DeployTaskId string `json:"deploy_task_id"`
	Message      string `json:"message"` // event message
	Reason       string `json:"reason"`  // event reason
}
