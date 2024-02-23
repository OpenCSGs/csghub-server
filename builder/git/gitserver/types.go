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

type UpdateRepoReq struct {
	Username      string               `json:"username" example:"creator_user_name"`
	Namespace     string               `json:"namespace" example:"user_or_org_name"`
	OriginName    string               `json:"origin_name"`
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

type DeleteRepoReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	RepoType  types.RepositoryType `json:"repo_type"`
}

type GetBranchesReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Per       int                  `json:"per"`
	Page      int                  `json:"page"`
	RepoType  types.RepositoryType `json:"repo_type"`
}

type GetRepoCommitsReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Per       int                  `json:"per"`
	Page      int                  `json:"page"`
	Ref       string               `json:"ref"`
	RepoType  types.RepositoryType `json:"repo_type"`
}
type GetRepoLastCommitReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Ref       string               `json:"ref"`
	RepoType  types.RepositoryType `json:"repo_type"`
}

type RepoBasicReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	RepoType  types.RepositoryType `json:"repo_type"`
}

type GetRepoInfoByPathReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Ref       string               `json:"ref"`
	Path      string               `json:"path"`
	RepoType  types.RepositoryType `json:"repo_type"`
}

type GetRepoTagsReq = GetBranchesReq
