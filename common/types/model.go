package types

type ModelDetail struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	Introduction  string `json:"introduction"`
	License       string `json:"license"`
	Private       bool   `json:"private"`
	DownloadCount int    `json:"download_count"`
	LastUpdatedAt string `json:"last_updated_at"`
	HTTPCloneURL  string `json:"http_clone_url"`
	SSHCloneURL   string `json:"ssh_clone_url"`
	Size          int    `json:"size"`
	DefaultBranch string `json:"default_branch"`
}

type ModelTag struct {
	Name    string         `json:"name"`
	Message string         `json:"message"`
	Commit  ModelTagCommit `json:"commit"`
}

type ModelTagCommit struct {
	ID string `json:"id"`
}

type ModelBranch struct {
	Name    string            `json:"name"`
	Message string            `json:"message"`
	Commit  ModelBranchCommit `json:"commit"`
}

type ModelBranchCommit struct {
	ID string `json:"id"`
}

type CreateModelReq struct {
	Username      string `json:"username"`
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Labels        string `json:"labels"`
	License       string `json:"license"`
	Readme        string `json:"readme"`
	DefaultBranch string `json:"default_branch"`
}

type UpdateModelReq struct {
	Namespace     string `json:"namespace"`
	OriginName    string `json:"origin_name"`
	Username      string `json:"username"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Labels        string `json:"labels"`
	License       string `json:"license"`
	Readme        string `json:"readme"`
	DefaultBranch string `json:"default_branch"`
}
