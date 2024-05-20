package types

type RepositoryType string

const (
	ResTypeKey  string = "hub-res-type"
	ResNameKey  string = "hub-res-name"
	ResDeployID string = "hub-deploy-id"

	ModelRepo   RepositoryType = "model"
	DatasetRepo RepositoryType = "dataset"
	SpaceRepo   RepositoryType = "space"
	CodeRepo    RepositoryType = "code"
	UnknownRepo RepositoryType = ""
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

// repo object(cover model/space/code/dataset) for deployer
type DeployRepo struct {
	DeployID         int64  `json:"deploy_id,omitempty"`
	DeployName       string `json:"deploy_name,omitempty"`
	SpaceID          int64  `json:"space_id,omitempty"`
	Namespace        string `json:"namespace,omitempty"`
	Name             string `json:"name,omitempty"`
	Status           int    `json:"status"`
	GitPath          string `json:"git_path,omitempty"`
	GitBranch        string `json:"git_branch,omitempty"`
	Sdk              string `json:"sdk,omitempty"`
	SdkVersion       string `json:"sdk_version,omitempty"`
	Env              string `json:"env,omitempty"`
	Secret           string `json:"secret,omitempty"`
	Template         string `json:"template,omitempty"`
	Hardware         string `json:"hardware,omitempty"`
	ImageID          string `json:"image_id,omitempty"`
	UserID           int64  `json:"user_id,omitempty"`
	ModelID          int64  `json:"model_id,omitempty"`
	RepoID           int64  `json:"repo_id,omitempty"`
	RuntimeFramework string `json:"runtime_framework,omitempty"`
	Annotation       string `json:"annotation,omitempty"`
	MinReplica       int    `json:"min_replica,omitempty"`
	MaxReplica       int    `json:"max_replica,omitempty"`
	SvcName          string `json:"svc_name,omitempty"`
	Endpoint         string `json:"end_point,omitempty"`
	Createtime       string `json:"create_time,omitempty"`
	Updatetime       string `json:"update_time,omitempty"`
	CostPerHour      int64  `json:"cost_per_hour,omitempty"`
	ClusterID        string `json:"cluster_id,omitempty"`
	SecureLevel      int    `json:"secure_level,omitempty"`
	Replica          int    `json:"replica,omitempty"`
}

type RuntimeFrameworkReq struct {
	FrameName    string `json:"frame_name"`
	FrameVersion string `json:"frame_version"`
	FrameImage   string `json:"frame_image"`
	Enabled      int64  `json:"enabled"`
}

type RuntimeFramework struct {
	ID           int64  `json:"id"`
	FrameName    string `json:"frame_name"`
	FrameVersion string `json:"frame_version"`
	FrameImage   string `json:"frame_image"`
	Enabled      int64  `json:"enabled"`
}
