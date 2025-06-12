package types

import "time"

const DefaultContainerPort = 8080

type SDKConfig struct {
	Name    string
	Version string
	Port    int
	Image   string
}

var (
	GRADIO = SDKConfig{
		Name:    "gradio",
		Version: "3.37.0",
		Port:    7860,
		Image:   "",
	}
	STREAMLIT = SDKConfig{
		Name:    "streamlit",
		Version: "1.33.0",
		Port:    8501,
		Image:   "",
	}
	NGINX = SDKConfig{
		Name:    "nginx",
		Version: "1.25.0",
		Port:    8000,
		Image:   "csg-nginx:1.2",
	}
	DOCKER = SDKConfig{
		Name:    "docker",
		Version: "",
		Port:    8080,
		Image:   "",
	}
	MCPSERVER = SDKConfig{
		Name:    "mcp_server",
		Version: "",
		Port:    8000,
		Image:   "",
	}
)

type MCPService struct {
	ID            int64     `json:"id,omitempty"`
	Creator       string    `json:"username,omitempty" example:"creator_user_name"`
	Name          string    `json:"name,omitempty" example:"mcp_name_1"`
	Nickname      string    `json:"nickname,omitempty" example:""`
	Description   string    `json:"description,omitempty" example:""`
	Path          string    `json:"path" example:"user_or_org_name/mcp_name_1"`
	License       string    `json:"license,omitempty" example:"MIT"`
	DefaultBranch string    `json:"default_branch,omitempty"`
	Private       bool      `json:"private"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
	Env           string    `json:"env,omitempty"`
	Secrets       string    `json:"secrets,omitempty"`
	Variables     string    `json:"variables,omitempty"`
	Endpoint      string    `json:"endpoint,omitempty" example:"https://localhost/spaces/myname/mymcp"`
	Status        string    `json:"status"`
	RepositoryID  int64     `json:"repository_id,omitempty"`
	SvcName       string    `json:"svc_name,omitempty"`
}

type CreateSpaceReq struct {
	CreateRepoReq
	Sdk           string `json:"sdk" example:"1" binding:"required"`
	SdkVersion    string `json:"sdk_version" example:"v0.1"`
	DriverVersion string `json:"driver_version" example:"11.8.0"`
	CoverImageUrl string `json:"cover_image_url"`
	Template      string `json:"template"`
	Env           string `json:"env"`
	Secrets       string `json:"secrets"`
	Variables     string `json:"variables"`
	ResourceID    int64  `json:"resource_id" binding:"required"`
	ClusterID     string `json:"cluster_id" binding:"required"`
	OrderDetailID int64  `json:"order_detail_id"`
}

// Space is the domain object for spaces
type Space struct {
	ID            int64       `json:"id,omitempty"`
	Creator       string      `json:"username,omitempty" example:"creator_user_name"`
	Name          string      `json:"name,omitempty" example:"space_name_1"`
	Nickname      string      `json:"nickname,omitempty" example:""`
	Description   string      `json:"description,omitempty" example:""`
	Path          string      `json:"path" example:"user_or_org_name/space_name_1"`
	License       string      `json:"license,omitempty" example:"MIT"`
	Tags          []RepoTag   `json:"tags,omitempty"`
	User          *User       `json:"user,omitempty"`
	Repository    *Repository `json:"repository,omitempty"`
	DefaultBranch string      `json:"default_branch,omitempty"`
	Likes         int64       `json:"like_count,omitempty"`
	Private       bool        `json:"private"`
	CreatedAt     time.Time   `json:"created_at,omitempty"`
	UpdatedAt     time.Time   `json:"updated_at,omitempty"`

	// like gradio,steamlit etc
	Sdk           string `json:"sdk,omitempty" example:"1"`
	SdkVersion    string `json:"sdk_version,omitempty" example:"v0.1"`
	DriverVersion string `json:"driver_version,omitempty" example:"11.8.0"`
	CoverImageUrl string `json:"cover_image_url,omitempty"`
	Template      string `json:"template,omitempty"`
	Env           string `json:"env,omitempty"`
	Hardware      string `json:"hardware,omitempty"`
	Secrets       string `json:"secrets,omitempty"`
	Variables     string `json:"variables,omitempty"`
	// the serving endpoint url
	Endpoint string `json:"endpoint,omitempty" example:"https://localhost/spaces/myname/myspace"`
	// deploying, running, failed
	Status               string               `json:"status"`
	RepositoryID         int64                `json:"repository_id,omitempty"`
	UserLikes            bool                 `json:"user_likes"`
	Source               RepositorySource     `json:"source"`
	SyncStatus           RepositorySyncStatus `json:"sync_status"`
	SKU                  string               `json:"sku,omitempty"`
	SvcName              string               `json:"svc_name,omitempty"`
	CanWrite             bool                 `json:"can_write"`
	CanManage            bool                 `json:"can_manage"`
	Namespace            *Namespace           `json:"namespace"`
	SensitiveCheckStatus string               `json:"sensitive_check_status"`
	DeployID             int64                `json:"deploy_id,omitempty"`
	Instances            []Instance           `json:"instances,omitempty"`
	ClusterID            string               `json:"cluster_id"`
}

type SpaceStatus struct {
	SvcName   string `json:"svc_name"`
	Status    string `json:"status"`
	DeployID  int64  `json:"deploy_id"`
	ClusterID string `json:"cluster_id"`
}

type UpdateSpaceReq struct {
	UpdateRepoReq
	Sdk           *string `json:"sdk" example:"1"`
	SdkVersion    *string `json:"sdk_version" example:"v0.1"`
	CoverImageUrl *string `json:"cover_image_url"`
	Template      *string `json:"template"`
	Env           *string `json:"env"`
	ResourceID    *int64  `json:"resource_id"`
	Secrets       *string `json:"secrets"`
	Variables     *string `json:"variables"`
	OrderDetailID int64   `json:"order_detail_id"`
	ClusterID     *string `json:"cluster_id"`
}
