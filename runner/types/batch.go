package types

import "opencsg.com/csghub-server/common/types"

// BatchStatusRequest is the request for the batch status API.
type BatchStatusRequest struct {
	ClusterID string             `json:"cluster_id"`
	Items     []BatchStatusItem  `json:"items"`
}

// BatchStatusItem identifies a single resource to check.
type BatchStatusItem struct {
	Type ResourceType `json:"type"`
	Name string       `json:"name"` // svc_name or sandbox_name
	ID   int64        `json:"id,omitempty"` // workflow ID
}

// ResourceType identifies the kind of resource.
type ResourceType string

const (
	ResourceTypeKsvc     ResourceType = "ksvc"
	ResourceTypeSandbox  ResourceType = "sandbox"
	ResourceTypeWorkflow ResourceType = "workflow"
)

// BatchStatusResponse is the response for the batch status API.
type BatchStatusResponse struct {
	Items []BatchStatusItemResult `json:"items"`
}

// BatchStatusItemResult holds the status of a single resource.
type BatchStatusItemResult struct {
	Type ResourceType `json:"type"`
	Name string       `json:"name"`

	// ksvc / sandbox
	Code      int              `json:"code,omitempty"`
	Instances []types.Instance `json:"instances,omitempty"`

	// ksvc
	ActualReplica  int `json:"actual_replica,omitempty"`
	DesiredReplica int `json:"desired_replica,omitempty"`

	// sandbox
	Status int `json:"status,omitempty"`

	// workflow
	Phase string `json:"phase,omitempty"`

	Error string `json:"error,omitempty"`
}
