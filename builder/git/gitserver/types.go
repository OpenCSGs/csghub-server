package gitserver

import "opencsg.com/csghub-server/common/types"

type CreateRepoReq struct {
	Username      string               `json:"username" example:"creator_user_name"`
	Namespace     string               `json:"namespace" example:"user_or_org_name"`
	Name          string               `json:"name" example:"model_name_1"`
	Nickname      string               `json:"nickname" example:"model display name"`
	Description   string               `json:"description"`
	Labels        string               `json:"labels" example:""`
	License       string               `json:"license" example:"MIT"`
	Readme        string               `json:"readme"`
	DefaultBranch string               `json:"default_branch" example:"main"`
	RepoType      types.RepositoryType `json:"type"`
	Private       bool                 `json:"private"`
}

type CreateRepoResp struct {
	Username      string               `json:"username" example:"creator_user_name"`
	Namespace     string               `json:"namespace" example:"user_or_org_name"`
	Name          string               `json:"name" example:"model_name_1"`
	Nickname      string               `json:"nickname" example:"model display name"`
	Description   string               `json:"description"`
	Labels        string               `json:"labels" example:""`
	License       string               `json:"license" example:"MIT"`
	Readme        string               `json:"readme"`
	DefaultBranch string               `json:"default_branch" example:"main"`
	RepoType      types.RepositoryType `json:"type"`
	GitPath       string               `json:"git_path"`
	SshCloneURL   string               `json:"ssh_clone_url"`
	HttpCloneURL  string               `json:"http_clone_url"`
	Private       bool                 `json:"private"`
}
