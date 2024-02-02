package types

type RepositoryType string

const (
	ModelRepo   RepositoryType = "model"
	DatasetRepo RepositoryType = "dataset"
	SpaceRepo   RepositoryType = "space"
	CodeRepo    RepositoryType = "code"
)

type RepoRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ref       string `json:"ref"`
	Path      string `json:"path"`
	Page      int    `json:"page"`
	Per       int    `json:"per"`
}
