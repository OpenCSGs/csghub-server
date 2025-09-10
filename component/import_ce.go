//go:build !ee && !saas

package component

import (
	"context"
	"errors"

	"opencsg.com/csghub-server/builder/importer"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

var ErrInvalidPath = errors.New("invalid path")
var ErrRepoAlreadyExists = errors.New("repository already exists")

type ImportComponent interface {
	Import(ctx context.Context, req types.ImportReq) error
	GetGitlabRepos(ctx context.Context, req *types.GetGitlabReposReq) ([]types.RemoteRepository, error)
	ImportStatus(ctx context.Context, req types.ImportStatusReq) ([]types.ImportedRepository, error)
}

type importComponentImpl struct {
	mirrorStore       database.MirrorStore
	repoStore         database.RepoStore
	userStore         database.UserStore
	codeStore         database.CodeStore
	importer          importer.Importer
	mirrorSourceStore database.MirrorSourceStore
	repoComponent     RepoComponent
	mirrorTaskStore   database.MirrorTaskStore
}

func NewImportComponentImpl(config *config.Config) (ImportComponent, error) {
	r, err := NewImportComponent(config)
	if err != nil {
		return nil, err
	}
	return r.(*importComponentImpl), nil
}

func NewImportComponent(config *config.Config) (ImportComponent, error) {
	c := &importComponentImpl{}
	return c, nil
}

func (c *importComponentImpl) Import(ctx context.Context, req types.ImportReq) error {
	return nil
}

func (c *importComponentImpl) ImportStatus(ctx context.Context, req types.ImportStatusReq) ([]types.ImportedRepository, error) {
	return nil, nil
}

func (c *importComponentImpl) GetGitlabRepos(ctx context.Context, req *types.GetGitlabReposReq) ([]types.RemoteRepository, error) {
	return nil, nil
}
