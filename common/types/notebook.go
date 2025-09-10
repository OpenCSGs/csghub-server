package types

import "time"

type CreateNotebookReq struct {
	CurrentUser        string `json:"currentUser"`
	DeployName         string `json:"deploy_name"`
	ResourceID         int64  `json:"resource_id"`
	MinReplica         int    `json:"min_replica" validate:"min=0"`
	RuntimeFrameworkID int64  `json:"runtime_framework_id"`
	OrderDetailID      int64  `json:"order_detail_id"`
}

type NotebookRes struct {
	ID                      int64     `json:"id"`
	CurrentUser             string    `json:"currentUser"`
	DeployName              string    `json:"deploy_name"`
	Status                  string    `json:"status"`
	ResourceID              string    `json:"resource_id"`
	ClusterID               string    `json:"cluster_id"`
	ResourceName            string    `json:"resource_name"`
	RuntimeFramework        string    `json:"runtime_framework"`
	RuntimeFrameworkVersion string    `json:"runtime_framework_version"`
	OrderDetailID           int64     `json:"order_detail_id"`
	PayMode                 PayMode   `json:"pay_mode,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
	Endpoint                string    `json:"endpoint"`
	SvcName                 string    `json:"svc_name"`
	SecureLevel             int       `json:"secure_level"`
}

type NotebookActionReq struct {
	ID          int64  `json:"id"`
	CurrentUser string `json:"currentUser"`
}

// Alias types for different actions, all share the same structure
type GetNotebookReq = NotebookActionReq
type DeleteNotebookReq = NotebookActionReq
type StopNotebookReq = NotebookActionReq
type StartNotebookReq = NotebookActionReq

type UpdateNotebookReq struct {
	ID          int64  `json:"id"`
	CurrentUser string `json:"currentUser"`
	ResourceID  int64  `json:"resource_id"`
}
