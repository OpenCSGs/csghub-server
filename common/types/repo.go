package types

import (
	"time"
)

type RepositoryType string
type RepositorySource string
type RepositorySyncStatus string

const (
	ResTypeKey  string = "hub-res-type"
	ResNameKey  string = "hub-res-name"
	ResDeployID string = "hub-deploy-id"

	ModelRepo   RepositoryType = "model"
	DatasetRepo RepositoryType = "dataset"
	SpaceRepo   RepositoryType = "space"
	CodeRepo    RepositoryType = "code"
	UnknownRepo RepositoryType = ""

	OpenCSGSource     RepositorySource = "opencsg"
	LocalSource       RepositorySource = "local"
	HuggingfaceSource RepositorySource = "huggingface"

	SyncStatusPending    RepositorySyncStatus = "pending"
	SyncStatusInProgress RepositorySyncStatus = "inprogress"
	SyncStatusFailed     RepositorySyncStatus = "failed"
	SyncStatusCompleted  RepositorySyncStatus = "completed"

	EndpointPublic  int = 1 // public - anyone can access
	EndpointPrivate int = 2 // private - access with read permission
)

type RepoRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ref       string `json:"ref"`
	Path      string `json:"path"`
	Page      int    `json:"page"`
	Per       int    `json:"per"`
}

type Branch struct {
	Name    string           `json:"name"`
	Message string           `json:"message"`
	Commit  RepoBranchCommit `json:"commit"`
}

type Tag struct {
	Name    string           `json:"name"`
	Message string           `json:"message"`
	Commit  DatasetTagCommit `json:"commit"`
}

type Repository struct {
	HTTPCloneURL string `json:"http_clone_url"`
	SSHCloneURL  string `json:"ssh_clone_url"`
}

type RepoPageOpts struct {
	PageOpts
	PageCount int `json:"page_count"`
	Total     int `json:"total"`
}

type Instance struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// repo object(cover model/space/code/dataset) for deployer
type DeployRepo struct {
	DeployID         int64      `json:"deploy_id,omitempty"`
	DeployName       string     `json:"deploy_name,omitempty"`
	SpaceID          int64      `json:"space_id,omitempty"`
	Path             string     `json:"model_id,omitempty"` // csghub ask for model_id = namespace/name
	Namespace        string     `json:"namespace,omitempty"`
	Name             string     `json:"name,omitempty"`
	Status           string     `json:"status"`
	GitPath          string     `json:"git_path,omitempty"`
	GitBranch        string     `json:"git_branch,omitempty"`
	Sdk              string     `json:"sdk,omitempty"`
	SdkVersion       string     `json:"sdk_version,omitempty"`
	Env              string     `json:"env,omitempty"`
	Secret           string     `json:"secret,omitempty"`
	Template         string     `json:"template,omitempty"`
	Hardware         string     `json:"hardware,omitempty"`
	ImageID          string     `json:"image_id,omitempty"`
	UserID           int64      `json:"user_id,omitempty"`
	ModelID          int64      `json:"repo_model_id,omitempty"` // for URM code logic
	RepoID           int64      `json:"repository_id,omitempty"`
	RuntimeFramework string     `json:"runtime_framework,omitempty"`
	ContainerPort    int        `json:"container_port,omitempty"`
	Annotation       string     `json:"annotation,omitempty"`
	MinReplica       int        `json:"min_replica,omitempty"`
	MaxReplica       int        `json:"max_replica,omitempty"`
	SvcName          string     `json:"svc_name,omitempty"`
	Endpoint         string     `json:"endpoint,omitempty"`
	CreatedAt        time.Time  `json:"created_at,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at,omitempty"`
	CostPerHour      float64    `json:"cost_per_hour,omitempty"`
	ClusterID        string     `json:"cluster_id,omitempty"`
	SecureLevel      int        `json:"secure_level,omitempty"`
	ActualReplica    int        `json:"actual_replica,omitempty"`
	DesiredReplica   int        `json:"desired_replica,omitempty"`
	Instances        []Instance `json:"instances,omitempty"`
	InstanceName     string     `json:"instance_name,omitempty"`
	Private          bool       `json:"private"`
	Type             int        `json:"type,omitempty"`
	ProxyEndpoint    string     `json:"proxy_endpoint,omitempty"`
	UserUUID         string     `json:"user_uuid,omitempty"`
	SKU              string     `json:"sku,omitempty"`
}

type RuntimeFrameworkReq struct {
	FrameName     string `json:"frame_name"`
	FrameVersion  string `json:"frame_version"`
	FrameImage    string `json:"frame_image"`
	FrameCpuImage string `json:"frame_cpu_image"`
	Enabled       int64  `json:"enabled"`
	ContainerPort int    `json:"container_port"`
	Type          int    `json:"type"`
}

type RuntimeFramework struct {
	ID            int64  `json:"id"`
	FrameName     string `json:"frame_name"`
	FrameVersion  string `json:"frame_version"`
	FrameImage    string `json:"frame_image"`
	FrameCpuImage string `json:"frame_cpu_image"`
	Enabled       int64  `json:"enabled"`
	ContainerPort int    `json:"container_port"`
	Type          int    `json:"type"`
}

type RuntimeFrameworkModels struct {
	Models []string `json:"models"`
}

type RepoFilter struct {
	Tags     []TagReq
	Sort     string
	Search   string
	Source   string
	Username string
}

type TagReq struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

type RuntimeArchitecture struct {
	Architectures []string `json:"architectures"`
}

type ScanReq struct {
	FrameID   int64
	FrameType int
	ArchMap   map[string]string
	Models    []string
}

type PermissionError struct {
	Message string
}

// Add the Error() method to PermissionError.
func (e *PermissionError) Error() string {
	return e.Message // Return the message field as the error description.
}
