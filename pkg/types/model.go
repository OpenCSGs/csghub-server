package types

type Model struct {
	UserID    string `json:"user_id"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Public    bool   `json:"public"`
}

type ModelDetail struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	Introduction  string `json:"introduction"`
	License       string `json:"license"`
	DownloadCount int    `json:"download_count"`
	LastUpdatedAt string `json:"last_updated_at"`
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
