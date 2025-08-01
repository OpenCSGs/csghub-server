package gitserver

import (
	"context"
	"io"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

const (
	Git_Header_X_Pagecount = "x-pagecount"
	Git_Header_X_Total     = "x-total"
)

type GitServer interface {
	CreateUser(CreateUserRequest) (*CreateUserResponse, error)
	// Depricated, will be removed in next version
	UpdateUser(*types.UpdateUserRequest, *database.User) (*database.User, error)
	UpdateUserV2(UpdateUserRequest) error
	CreateUserToken(*types.CreateUserTokenRequest) (*database.AccessToken, error)
	DeleteUserToken(*types.DeleteUserTokenRequest) error

	RepositoryExists(ctx context.Context, req CheckRepoReq) (bool, error)
	GetRepo(ctx context.Context, req GetRepoReq) (*CreateRepoResp, error)
	CreateRepo(ctx context.Context, req CreateRepoReq) (*CreateRepoResp, error)
	UpdateRepo(ctx context.Context, req UpdateRepoReq) (*CreateRepoResp, error)
	DeleteRepo(ctx context.Context, req DeleteRepoReq) error
	GetRepoBranches(ctx context.Context, req GetBranchesReq) ([]types.Branch, error)
	GetRepoBranchByName(ctx context.Context, req GetBranchReq) (*types.Branch, error)
	DeleteRepoBranch(ctx context.Context, req DeleteBranchReq) error
	GetRepoCommits(ctx context.Context, req GetRepoCommitsReq) ([]types.Commit, *types.RepoPageOpts, error)
	GetRepoLastCommit(ctx context.Context, req GetRepoLastCommitReq) (*types.Commit, error)
	GetSingleCommit(ctx context.Context, req GetRepoLastCommitReq) (*types.CommitResponse, error)
	GetCommitDiff(ctx context.Context, req GetRepoLastCommitReq) ([]byte, error)
	GetRepoFileTree(ctx context.Context, req GetRepoInfoByPathReq) ([]*types.File, error)
	GetTree(ctx context.Context, req types.GetTreeRequest) (*types.GetRepoFileTreeResp, error)
	GetLogsTree(ctx context.Context, req types.GetLogsTreeRequest) (*types.LogsTreeResp, error)
	GetRepoFileRaw(ctx context.Context, req GetRepoInfoByPathReq) (string, error)
	GetRepoFileReader(ctx context.Context, req GetRepoInfoByPathReq) (io.ReadCloser, int64, error)
	GetRepoLfsFileRaw(ctx context.Context, req GetRepoInfoByPathReq) (io.ReadCloser, error)
	GetRepoFileContents(ctx context.Context, req GetRepoInfoByPathReq) (*types.File, error)
	CreateRepoFile(req *types.CreateFileReq) (err error)
	UpdateRepoFile(req *types.UpdateFileReq) (err error)
	DeleteRepoFile(req *types.DeleteFileReq) (err error)
	GetRepoAllFiles(ctx context.Context, req GetRepoAllFilesReq) ([]*types.File, error)
	GetRepoAllLfsPointers(ctx context.Context, req GetRepoAllFilesReq) ([]*types.LFSPointer, error)
	GetDiffBetweenTwoCommits(ctx context.Context, req GetDiffBetweenTwoCommitsReq) (*types.GiteaCallbackPushReq, error)

	CreateSSHKey(*types.CreateSSHKeyRequest) (*database.SSHKey, error)
	// ListSSHKeys(string, int, int) ([]*database.SSHKey, error)
	DeleteSSHKey(int) error

	CreateOrganization(req *types.CreateOrgReq, orgOwner database.User) (*database.Organization, error)
	DeleteOrganization(string) error
	UpdateOrganization(*types.EditOrgReq, *database.Organization) (*database.Organization, error)

	FixOrganization(req *types.CreateOrgReq, orgOwner database.User) error
	FixUserData(ctx context.Context, userName string) error

	// Mirror
	// CreateMirrorRepo creates a mirror repository and returns a gitea task id
	CreateMirrorRepo(ctx context.Context, req CreateMirrorRepoReq) (int64, error)
	// GetMirrorTaskInfo returns the Gitea mirror task info
	GetMirrorTaskInfo(ctx context.Context, taskId int64) (*MirrorTaskInfo, error)
	// MirrorSync requests the Gitea to start mirror synchronization
	MirrorSync(ctx context.Context, req MirrorSyncReq) error

	// For gitaly smart http methods
	InfoRefsResponse(ctx context.Context, req InfoRefsReq) (io.Reader, error)
	// Handle git clone or fetch request
	UploadPack(ctx context.Context, req UploadPackReq) error
	// Handle git push request
	ReceivePack(ctx context.Context, req ReceivePackReq) error
	CommitFiles(ctx context.Context, req CommitFilesReq) error
	BuildRelativePath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (string, error)
	UpdateRef(ctx context.Context, req UpdateRefReq) error
}
