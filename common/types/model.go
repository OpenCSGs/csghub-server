package types

import (
	"time"
)

type RepoBranchCommit struct {
	ID string `json:"id"`
}

type CreateModelReq struct {
	CreateRepoReq
}

type UpdateModelReq struct {
	CreateRepoReq
}

type UpdateDownloadsReq struct {
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	ReqDate    string `json:"date"`
	Date       time.Time
	CloneCount int64 `json:"download_count"`
	RepoType   RepositoryType
}

type ModelPredictReq struct {
	Namespace   string `json:"-"`
	Name        string `json:"-"`
	Input       string `json:"input"`
	Version     string `json:"version"`
	CurrentUser string `json:"current_user"`
}

type ModelPredictResp struct {
	Content string `json:"content"`
	// TODO:add metrics like tokens, latency etc
}

type CreateRepoReq struct {
	Username      string         `json:"-" example:"creator_user_name"`
	Namespace     string         `json:"namespace" example:"user_or_org_name"`
	Name          string         `json:"name" example:"model_name_1"`
	Nickname      string         `json:"nickname" example:"model display name"`
	Description   string         `json:"description"`
	Private       bool           `json:"private"`
	Labels        string         `json:"labels" example:""`
	License       string         `json:"license" example:"MIT"`
	Readme        string         `json:"readme"`
	DefaultBranch string         `json:"default_branch" example:"main"`
	RepoType      RepositoryType `json:"-"`
}

type DeleteRepoReq struct {
	Username  string         `json:"username" example:"creator_user_name"`
	Namespace string         `json:"namespace" example:"user_or_org_name"`
	Name      string         `json:"name" example:"model_name_1"`
	RepoType  RepositoryType `json:"-"`
}

type Relations struct {
	Models   []*Model   `json:"models,omitempty"`
	Datasets []*Dataset `json:"datasets,omitempty"`
	Codes    []*Code    `json:"codes,omitempty"`
	Spaces   []*Space   `json:"spaces,omitempty"`
}

type Model struct {
	ID            int64      `json:"id,omitempty"`
	Name          string     `json:"name"`
	Nickname      string     `json:"nickname"`
	Description   string     `json:"description"`
	Likes         int64      `json:"likes"`
	Downloads     int64      `json:"downloads"`
	Path          string     `json:"path"`
	RepositoryID  int64      `json:"repository_id"`
	Private       bool       `json:"private"`
	User          *User      `json:"user,omitempty"`
	Tags          []RepoTag  `json:"tags,omitempty"`
	Readme        string     `json:"readme"`
	Repository    Repository `json:"repository"`
	DefaultBranch string     `json:"default_branch"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	// widget UI style: generation,chat
	WidgetType ModelWidgetType `json:"widget_type" example:"generation"`
	// url to interact with the model
	Status          string               `json:"status" example:"RUNNING"`
	UserLikes       bool                 `json:"user_likes"`
	Source          RepositorySource     `json:"source"`
	SyncStatus      RepositorySyncStatus `json:"sync_status"`
	EnableInference bool                 `json:"enable_inference"`
	EnableFinetune  bool                 `json:"enable_finetune"`
}

type SDKModelInfo struct {
	ID     string `json:"id"`
	Author string `json:"author,omitempty"`
	// last commit sha
	Sha              string                 `json:"sha,omitempty"`
	CreatedAt        time.Time              `json:"created_at,omitempty"`
	LastModified     time.Time              `json:"last_modified,omitempty"`
	Private          bool                   `json:"private"`
	Disabled         bool                   `json:"disabled,omitempty"`
	Gated            interface{}            `json:"gated,omitempty"` // "auto", "manual", or false
	Downloads        int                    `json:"downloads"`
	Likes            int                    `json:"likes"`
	LibraryName      string                 `json:"library_name,omitempty"`
	Tags             []string               `json:"tags"`
	PipelineTag      string                 `json:"pipeline_tag,omitempty"`
	MaskToken        string                 `json:"mask_token,omitempty"`
	WidgetData       interface{}            `json:"widget_data,omitempty"`       // Type Any
	ModelIndex       map[string]interface{} `json:"model_index,omitempty"`       // Dict
	Config           map[string]interface{} `json:"config,omitempty"`            // Dict
	TransformersInfo interface{}            `json:"transformers_info,omitempty"` // TransformersInfo
	CardData         interface{}            `json:"card_data,omitempty"`         // ModelCardData
	Siblings         []SDKFile              `json:"siblings"`
	Spaces           []string               `json:"spaces,omitempty"`
	SafeTensors      interface{}            `json:"safetensors,omitempty"` // SafeTensorsInfo
}

type ModelWidgetType string

const (
	ModelWidgetTypeGeneration ModelWidgetType = "generation"
	ModelWidgetTypeChat       ModelWidgetType = "chat"
)

type ModelRunReq struct {
	DeployName         string `json:"deploy_name"`
	ClusterID          string `json:"cluster_id"`
	Env                string `json:"env"`
	ResourceID         int64  `json:"resource_id"`
	RuntimeFrameworkID int64  `json:"runtime_framework_id"`
	MinReplica         int    `json:"min_replica"`
	MaxReplica         int    `json:"max_replica"`
	Revision           string `json:"revision"`
	SecureLevel        int    `json:"secure_level"`
}

type InstanceRunReq struct {
	DeployName         string `json:"deploy_name"`
	ClusterID          string `json:"cluster_id"`
	ResourceID         int64  `json:"resource_id"`
	RuntimeFrameworkID int64  `json:"runtime_framework_id"`
	Revision           string `json:"revision"`
}

type ModelUpdateRequest struct {
	MinReplica int               `json:"min_replica"` // min replica of instance/pod
	MaxReplica int               `json:"max_replica"` // max replica of instance/pod
	Hardware   HardWare          `json:"hardware"`    // resource requirements
	ImageID    string            `json:"image_id" binding:"required"`
	Env        map[string]string `json:"env"` // runtime env variables
	ClusterID  string            `json:"cluster_id"`
	SvcName    string            `json:"svc_name"`
}

type ModelUpdateResponse struct {
	DeployID int64  `json:"deploy_id"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
}

type ModelStatusEventData struct {
	Status  string     `json:"status"`
	Details []Instance `json:"details"`
}

const (
	SpaceType      = 0 // space
	InferenceType  = 1 // inference endpoint
	FinetuneType   = 2 // finetune
	ServerlessType = 3 // serverless
)

type DeployActReq struct {
	RepoType     RepositoryType `json:"repo_type"`
	Namespace    string         `json:"namespace"`
	Name         string         `json:"name"`
	CurrentUser  string         `json:"current_user"`
	DeployID     int64          `json:"deploy_id"`
	DeployType   int            `json:"deploy_type"`
	InstanceName string         `json:"instance_name"`
}
