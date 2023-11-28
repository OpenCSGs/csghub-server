package gitserver

import (
	"errors"

	"git-devops.opencsg.com/product/community/starhub-server/config"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/gitserver/gitea"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
)

type GitServer interface {
	CreateUser(*types.CreateUserRequest) (*database.User, error)
	UpdateUser(*types.UpdateUserRequest) (int, *database.User, error)
	CreateModelRepo(*types.CreateModelReq) (*types.Model, error)
	UpdateModelRepo(*types.Model) (*types.Model, error)
	GetModelBranches(*types.RepoRequest) ([]*types.ModelBranch, error)
	GetModelCommits(*types.RepoRequest) ([]*types.Commit, error)
	GetModelLastCommit(*types.RepoRequest) (*types.Commit, error)
	GetModelDetail(*types.RepoRequest) (*types.ModelDetail, error)
	GetModelFileRaw(*types.RepoRequest) (string, error)
	GetModelTags(*types.RepoRequest) ([]*types.ModelTag, error)
	GetModelFileTree(*types.RepoRequest) ([]*types.File, error)

	CreateDatasetRepo(*types.CreateModelReq) (*types.Dataset, error)
	UpdateDatasetRepo(*types.Dataset) (*types.Dataset, error)
	GetDatasetBranches(*types.RepoRequest) ([]*types.DatasetBranch, error)
	GetDatasetCommits(*types.RepoRequest) ([]*types.Commit, error)
	GetDatasetLastCommit(*types.RepoRequest) (*types.Commit, error)
	GetDatasetDetail(*types.RepoRequest) (*types.DatasetDetail, error)
	GetDatasetFileRaw(*types.RepoRequest) (string, error)
	GetDatasetTags(*types.RepoRequest) ([]*types.DatasetTag, error)
	GetDatasetFileTree(*types.RepoRequest) ([]*types.File, error)
}

func NewGitServer(config *config.Config) (GitServer, error) {
	if config.GitServer.Type == "gitea" {
		gitServer, err := gitea.NewClient(config)
		return gitServer, err
	}

	return nil, errors.New("Undefined git server type.")
}
