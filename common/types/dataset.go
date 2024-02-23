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

type Tag struct {
	Name    string           `json:"name"`
	Message string           `json:"message"`
	Commit  DatasetTagCommit `json:"commit"`
}

type DatasetTagCommit struct {
	ID string `json:"id"`
}

type DatasetBranch struct {
	Name    string           `json:"name"`
	Message string           `json:"message"`
	Commit  RepoBranchCommit `json:"commit"`
}

type CreateDatasetReq struct {
	CreateRepoReq
}

type UpdateDatasetReq struct {
	CreateRepoReq
}
