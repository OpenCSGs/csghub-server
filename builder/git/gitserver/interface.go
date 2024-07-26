package gitserver

import (
	"context"
	"io"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
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

	GetRepo(ctx context.Context, req GetRepoReq) (*CreateRepoResp, error)
	CreateRepo(ctx context.Context, req CreateRepoReq) (*CreateRepoResp, error)
	UpdateRepo(ctx context.Context, req UpdateRepoReq) (*CreateRepoResp, error)
	DeleteRepo(ctx context.Context, req DeleteRepoReq) error
	GetRepoBranches(ctx context.Context, req GetBranchesReq) ([]types.Branch, error)
	GetRepoCommits(ctx context.Context, req GetRepoCommitsReq) ([]types.Commit, *types.RepoPageOpts, error)
	GetRepoLastCommit(ctx context.Context, req GetRepoLastCommitReq) (*types.Commit, error)
	GetSingleCommit(ctx context.Context, req GetRepoLastCommitReq) (*gitea.Commit, error)
	GetCommitDiff(ctx context.Context, req GetRepoLastCommitReq) ([]byte, error)
	GetRepoFileTree(ctx context.Context, req GetRepoInfoByPathReq) ([]*types.File, error)
	GetRepoFileRaw(ctx context.Context, req GetRepoInfoByPathReq) (string, error)
	GetRepoFileReader(ctx context.Context, req GetRepoInfoByPathReq) (io.ReadCloser, int64, error)
	GetRepoLfsFileRaw(ctx context.Context, req GetRepoInfoByPathReq) (io.ReadCloser, error)
	GetRepoFileContents(ctx context.Context, req GetRepoInfoByPathReq) (*types.File, error)
	CreateRepoFile(req *types.CreateFileReq) (err error)
	UpdateRepoFile(req *types.UpdateFileReq) (err error)

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
}
