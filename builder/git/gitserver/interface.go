package gitserver

import (
	"context"
	"io"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type GitServer interface {
	CreateUser(*types.CreateUserRequest) (*database.User, error)
	UpdateUser(*types.UpdateUserRequest, *database.User) (*database.User, error)
	CreateUserToken(*types.CreateUserTokenRequest) (*database.AccessToken, error)
	DeleteUserToken(*types.DeleteUserTokenRequest) error

	CreateRepo(ctx context.Context, req CreateRepoReq) (*CreateRepoResp, error)

	CreateModelRepo(*types.CreateModelReq) (*database.Model, *database.Repository, error)
	UpdateModelRepo(string, string, *database.Model, *database.Repository, *types.UpdateModelReq) error
	DeleteModelRepo(string, string) error
	GetModelBranches(string, string, int, int) ([]*types.ModelBranch, error)
	GetModelCommits(string, string, string, int, int) ([]*types.Commit, error)
	GetModelLastCommit(string, string, string) (*types.Commit, error)
	GetModelDetail(namespace, name string) (*types.ModelDetail, error)
	GetModelFileRaw(namespace, repoName, ref, filePath string) (string, error)
	GetModelFileReader(namespace, repoName, ref, filePath string) (io.ReadCloser, error)
	GetModelLfsFileRaw(namespace, repoName, ref, filePath string) (io.ReadCloser, error)
	GetModelTags(string, string, int, int) ([]*types.ModelTag, error)
	GetModelFileTree(string, string, string, string) ([]*types.File, error)
	CreateModelFile(*types.CreateFileReq) (err error)
	UpdateModelFile(*types.UpdateFileReq) (err error)

	CreateDatasetRepo(*types.CreateDatasetReq) (*database.Dataset, *database.Repository, error)
	UpdateDatasetRepo(string, string, *database.Dataset, *database.Repository, *types.UpdateDatasetReq) error
	DeleteDatasetRepo(string, string) error
	GetDatasetBranches(string, string, int, int) ([]*types.DatasetBranch, error)
	GetDatasetCommits(string, string, string, int, int) ([]*types.Commit, error)
	GetDatasetLastCommit(string, string, string) (*types.Commit, error)
	GetDatasetDetail(namespace, name string) (*types.DatasetDetail, error)
	GetDatasetFileRaw(namespace, repoName, ref, filePath string) (string, error)
	GetDatasetFileReader(namespace, repoName, ref, filePath string) (io.ReadCloser, error)
	GetDatasetLfsFileRaw(namespace, repoName, ref, filePath string) (io.ReadCloser, error)
	GetDatasetTags(string, string, int, int) ([]*types.DatasetTag, error)
	GetDatasetFileTree(string, string, string, string) ([]*types.File, error)
	GetDatasetFileContents(namespace, repo, ref, path string) (*types.File, error)
	CreateDatasetFile(*types.CreateFileReq) (err error)
	UpdateDatasetFile(*types.UpdateFileReq) (err error)

	CreateSSHKey(*types.CreateSSHKeyRequest) (*database.SSHKey, error)
	// ListSSHKeys(string, int, int) ([]*database.SSHKey, error)
	DeleteSSHKey(int) error

	CreateOrganization(req *types.CreateOrgReq, orgOwner database.User) (*database.Organization, error)
	DeleteOrganization(string) error
	UpdateOrganization(*types.EditOrgReq, *database.Organization) (*database.Organization, error)

	FixOrganization(req *types.CreateOrgReq, orgOwner database.User) error
}
