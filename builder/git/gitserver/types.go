package gitserver

import (
	"net/http"

	"opencsg.com/csghub-server/common/types"
)

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

// CheckRepoReq identifies a repository whose storage existence should be checked.
type CheckRepoReq struct {
	RepoType  types.RepositoryType `json:"type"`
	Namespace string               `json:"namespace" example:"user_or_org_name"`
	Name      string               `json:"name" example:"model_name_1"`
	// RelativePath bypasses repository metadata lookup when the caller already knows the Gitaly path.
	RelativePath string `json:"-"`
}

// CreateRepoReq contains repository metadata required to create Git storage.
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
	// RelativePath bypasses repository metadata lookup when the caller already knows the Gitaly path.
	RelativePath string `json:"-"`
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

// DeleteRepoReq identifies a repository for lookup or deletion operations.
type DeleteRepoReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	RepoType  types.RepositoryType `json:"repo_type"`
	// RelativePath bypasses repository metadata lookup when the caller already knows the Gitaly path.
	RelativePath string `json:"-"`
}

type GetRepoReq = DeleteRepoReq

type GetBranchesReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Per       int                  `json:"per"`
	Page      int                  `json:"page"`
	RepoType  types.RepositoryType `json:"repo_type"`
}

type CreateBranchReq struct {
	Namespace  string               `json:"namespace"`
	Name       string               `json:"name"`
	BranchName string               `json:"branch_name"`
	CommitID   string               `json:"commit_id"`
	RepoType   types.RepositoryType `json:"repo_type"`
}

type SetDefaultBranchReq struct {
	Namespace  string               `json:"namespace"`
	Name       string               `json:"name"`
	BranchName string               `json:"branch_name"`
	RepoType   types.RepositoryType `json:"repo_type"`
}

type GetBranchReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Ref       string               `json:"ref"`
	RepoType  types.RepositoryType `json:"repo_type"`
}

type DeleteBranchReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Ref       string               `json:"ref"`
	RepoType  types.RepositoryType `json:"repo_type"`
	Username  string               `json:"username"`
	Email     string               `json:"email"`
}

type GetRepoCommitsReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Per       int                  `json:"per"`
	Page      int                  `json:"page"`
	Ref       string               `json:"ref"`
	RepoType  types.RepositoryType `json:"repo_type"`
}

// GetRepoLastCommitReq identifies a repository revision whose latest commit is requested.
type GetRepoLastCommitReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Ref       string               `json:"ref"`
	RepoType  types.RepositoryType `json:"repo_type"`
	// RelativePath bypasses repository metadata lookup when the caller already knows the Gitaly path.
	RelativePath string `json:"-"`
}

type GetArchiveReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Revision  string               `json:"revision"`
	RepoType  types.RepositoryType `json:"repo_type"`
}

// GetDiffBetweenTwoCommitsReq identifies the repository and revisions used to build a push callback.
type GetDiffBetweenTwoCommitsReq struct {
	Namespace     string               `json:"namespace"`
	Name          string               `json:"name"`
	Ref           string               `json:"ref"`
	RepoType      types.RepositoryType `json:"repo_type"`
	LeftCommitId  string               `json:"left_commit_id"`
	RightCommitId string               `json:"right_commit_id"`
	Private       bool                 `json:"private"`
	// RelativePath bypasses repository metadata lookup when the caller already knows the Gitaly path.
	RelativePath string `json:"-"`
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
	File      bool                 `json:"file"`
	// limit file size, don't return file content if file size is greater than MaxFileSize
	MaxFileSize                           int64    `json:"max_file_size"`
	GitObjectDirectoryRelative            string   `json:"git_object_directory_relative"`
	GitAlternateObjectDirectoriesRelative []string `json:"git_alternate_object_directories_relative"`
}

// GetRepoAllFilesReq identifies a repository revision for full file or LFS pointer scans.
type GetRepoAllFilesReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	Ref       string               `json:"ref"`
	RepoType  types.RepositoryType `json:"repo_type"`
	// RelativePath bypasses repository metadata lookup when the caller already knows the Gitaly path.
	RelativePath string `json:"-"`
}

type GetRepoTagsReq = GetBranchesReq

type CreateMirrorRepoReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	CloneUrl    string `json:"clone_url"`
	Username    string `json:"username"`
	AccessToken string `json:"access_token"`
	Private     bool   `json:"private"`
	Description string `json:"description"`
	MirrorToken string `json:"mirror_token"`
	RepoType    types.RepositoryType
}

// MirrorSyncReq contains the local repository path and remote credentials for a mirror fetch.
type MirrorSyncReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	RepoType    types.RepositoryType
	CloneUrl    string `json:"clone_url"`
	Username    string `json:"username"`
	AccessToken string `json:"access_token"`
	MirrorToken string `json:"mirror_token"`
	// RelativePath bypasses repository metadata lookup when the caller already knows the Gitaly path.
	RelativePath string `json:"-"`
}

type InfoRefsReq struct {
	Namespace   string               `json:"namespace"`
	Name        string               `json:"name"`
	RepoType    types.RepositoryType `json:"repo_type"`
	Rpc         string               `json:"rpc"`
	GitProtocol string               `json:"git_protocol"`
}

type UploadPackReq struct {
	Namespace   string               `json:"namespace"`
	Name        string               `json:"name"`
	RepoType    types.RepositoryType `json:"repo_type"`
	GitProtocol string               `json:"git_protocol"`
	Request     *http.Request        `json:"reader"`
	Writer      http.ResponseWriter  `json:"writer"`
	UserId      int64                `json:"user_id"`
	Username    string               `json:"username"`
}

type ReceivePackReq = UploadPackReq

type GetRepoFilesReq struct {
	Namespace                             string               `json:"namespace"`
	Name                                  string               `json:"name"`
	Ref                                   string               `json:"ref"`
	RepoType                              types.RepositoryType `json:"repo_type"`
	Revisions                             []string             `json:"revision"`
	GitObjectDirectoryRelative            string               `json:"git_object_directory_relative"`
	GitAlternateObjectDirectoriesRelative []string             `json:"git_alternate_object_directories_relative"`
}

type CommitFilesReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	RepoType  types.RepositoryType `json:"repo_type"`
	Revision  string               `json:"revision"`
	Username  string               `json:"username"`
	Email     string               `json:"email"`
	Message   string               `json:"message"`
	Files     []CommitFile         `json:"files"`
}

type CommitFile struct {
	Path    string       `json:"path"`
	Content string       `json:"content"`
	Action  CommitAction `json:"action"`
}

type CommitAction string

const (
	CommitActionCreate CommitAction = "create"
	CommitActionUpdate CommitAction = "update"
	CommitActionDelete CommitAction = "delete"
)

// UpdateRefReq describes one repository reference update.
type UpdateRefReq struct {
	Namespace   string               `json:"namespace"`
	Name        string               `json:"name"`
	RepoType    types.RepositoryType `json:"repo_type"`
	Ref         string               `json:"ref"`
	OldObjectId string               `json:"old_object_id"`
	NewObjectId string               `json:"new_object_id"`
	// RelativePath bypasses repository metadata lookup when the caller already knows the Gitaly path.
	RelativePath string `json:"-"`
}

type CopyRepositoryReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	RepoType  types.RepositoryType `json:"repo_type"`
	NewPath   string               `json:"new_path"`
}

type ReplicateRepositoryReq struct {
	FromRelativePath string `json:"from_relative_path"`
	ToRelativePath   string `json:"to_relative_path"`
}

type GetFilesByRevisionAndPathsReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	RepoType  types.RepositoryType `json:"repo_type"`
	Revision  string               `json:"revision"`
	Paths     []string             `json:"paths"`
}

type CreateForkReq struct {
	// Source repository information
	SourceRepoType  types.RepositoryType `json:"source_repo_type"`
	SourceNamespace string               `json:"source_namespace"`
	SourceName      string               `json:"source_name"`
	// Target repository information
	TargetRepoType  types.RepositoryType `json:"target_repo_type"`
	TargetNamespace string               `json:"target_namespace"`
	TargetName      string               `json:"target_name"`
	// Revision to fork from (optional)
	Revision string `json:"revision"`
}
