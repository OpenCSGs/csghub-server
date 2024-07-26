package gitserver

import "opencsg.com/csghub-server/common/types"

type CreateUserRequest struct {
	// Display name of the user
	Nickname string `json:"name"`
	// the login name
	Username string `json:"username"`
	Email    string `json:"email"`
}

type CreateUserResponse struct {
	// Display name of the user
	NickName string `json:"name"`
	// the login name
	Username string `json:"username"`
	Email    string `json:"email"`
	GitID    int64  `json:"git_id"`
	Password string `json:"-"`
}

type UpdateUserRequest struct {
	// Display name of the user
	Nickname *string `json:"name"`
	// the login name
	Username string  `json:"username"`
	Email    *string `json:"email"`
}

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

type GetRepoReq = DeleteRepoReq

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

const (
	TaskStatusQueued   TaskStatus = iota // 0 task is queued
	TaskStatusRunning                    // 1 task is running
	TaskStatusStopped                    // 2 task is stopped (never used)
	TaskStatusFailed                     // 3 task is failed
	TaskStatusFinished                   // 4 task is finished
)

type CreateMirrorRepoReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	CloneUrl    string `json:"clone_url"`
	Username    string `json:"username"`
	AccessToken string `json:"access_token"`
	Private     bool   `json:"private"`
	Description string `json:"description"`
	Interval    string `json:"interval"`
	MirrorToken string `json:"mirror_token"`
	RepoType    types.RepositoryType
}

type MirrorSyncReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	RepoType  types.RepositoryType
}

type MirrorTaskInfo struct {
	Status    TaskStatus `json:"status"`
	Message   string     `json:"message"`
	RepoID    int64      `json:"repo_id"`
	RepoName  string     `json:"repo_name"`
	StartedAt int64      `json:"start"`
	EndedAt   int64      `json:"end"`
}

type TaskStatus int
