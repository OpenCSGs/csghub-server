package types

type ImportReq struct {
	ImportRepos []ImportBaseReq `json:"import_repos"`
	CurrentUser string          `json:"-"`
	BaseURL     string          `json:"base_url" validate:"required,url"`
	AccessToken string          `json:"access_token" validate:"required"`
}

type ImportSingleRepoReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Username    string `json:"username"`
	Private     bool   `json:"private"`
	SourcePath  string `json:"source_path"`
	BaseURL     string `json:"base_url" validate:"required,url"`
	AccessToken string `json:"access_token" validate:"required"`
}

type ImportBaseReq struct {
	Path       string `json:"path"`
	SourcePath string `json:"source_path"`
	Private    bool   `json:"private"`
}

type GetGitlabReposReq struct {
	CurrentUser string
	BaseURL     string
	AccessToken string
	Search      string
	Per         int
	Page        int
}

type ImportStatusReq struct {
	CurrentUser string `json:"-"`
	Per         int    `json:"per"`
	Page        int    `json:"page"`
}

type ImportedRepository struct {
	SourcePath string `json:"source_path"`
	LocalPath  string `json:"local_path"`
	Status     string `json:"status"`
}

type ImportStatusResp struct {
	ImportedRepositories []ImportedRepository `json:"imported_projects"`
	RemoteRepositories   []RemoteRepository   `json:"remote_repositories"`
}

type RemoteRepository struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	SourceURL string `json:"source_url"`
	Private   bool   `json:"private"`
}
