package types

type ImageBuilderRequest struct {
	ClusterID      string `json:"cluster_id"`
	SpaceName      string `json:"space_name"`
	OrgName        string `json:"org_name"`
	SpaceURL       string `json:"space_url"`
	Sdk            string `json:"sdk"`
	Sdk_version    string `json:"sdk_version"`
	PythonVersion  string `json:"python_version"`
	Hardware       string `json:"hardware,omitempty"`
	DriverVersion  string `json:"driver_version,omitempty"`
	FactoryBuild   bool   `json:"factory_build,omitempty"`
	GitRef         string `json:"git_ref"`
	UserId         string `json:"user_id"`
	GitAccessToken string `json:"git_access_token"`
	LastCommitID   string `json:"last_commit_id"`
	DeployId       string `json:"deploy_id,omitempty"`
	TaskId         int64  `json:"task_id,omitempty"`
}

type ImageBuilderEvent struct {
	DeployId   string `json:"deploy_id,omitempty"`
	TaskId     int64  `json:"task_id,omitempty"`
	Status     string `json:"status,omitempty"`
	Message    string `json:"message,omitempty"`
	ImagetPath string `json:"imaget_path"`
}

type ImageBuildStopReq struct {
	OrgName   string `json:"org_name"`
	SpaceName string `json:"space_name"`
	DeployId  string `json:"deploy_id"`
	TaskId    string `json:"task_id"`
	ClusterID string `json:"cluster_id"`
}

type ImageBuildResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
