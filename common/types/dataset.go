package types

type DatasetDetail struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	Nickname      string `json:"nickname"`
	Introduction  string `json:"introduction"`
	License       string `json:"license"`
	Private       bool   `json:"private"`
	Downloads     int    `json:"downloads"`
	LastUpdatedAt string `json:"last_updated_at"`
	HTTPCloneURL  string `json:"http_clone_url"`
	SSHCloneURL   string `json:"ssh_clone_url"`
	Size          int    `json:"size"`
	DefaultBranch string `json:"default_branch"`
}

type DatasetTag struct {
	Name    string           `json:"name"`
	Message string           `json:"message"`
	Commit  DatasetTagCommit `json:"commit"`
}

type DatasetTagCommit struct {
	ID string `json:"id"`
}

type DatasetBranch struct {
	Name    string              `json:"name"`
	Message string              `json:"message"`
	Commit  DatasetBranchCommit `json:"commit"`
}

type DatasetBranchCommit struct {
	ID string `json:"id"`
}

type CreateDatasetReq struct {
	Username      string `json:"username" example:"creator_user_name"`
	Namespace     string `json:"namespace" example:"user_or_org_name"`
	Name          string `json:"name" example:"dataset_name"`
	Nickname      string `json:"nickname" example:"data set display name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Labels        string `json:"labels" example:""`
	License       string `json:"license" example:"MIT"`
	Readme        string `json:"readme"`
	DefaultBranch string `json:"default_branch" example:"main"`
}

type UpdateDatasetReq struct {
	Namespace     string `json:"namespace"`
	Username      string `json:"username"`
	OriginName    string `json:"origin_name"`
	Name          string `json:"name"`
	Nickname      string `json:"nickname"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Labels        string `json:"labels"`
	License       string `json:"license"`
	Readme        string `json:"readme"`
	DefaultBranch string `json:"default_branch"`
}
