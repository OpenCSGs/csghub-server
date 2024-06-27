package types

type DeployReq struct {
	CurrentUser string `json:"current_user"`
	PageOpts
	RepoType   RepositoryType `json:"repo_type"`
	DeployType int            `json:"deploy_type"`
}
