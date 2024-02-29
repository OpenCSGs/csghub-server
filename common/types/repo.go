package types

type RepositoryType string

const (
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
