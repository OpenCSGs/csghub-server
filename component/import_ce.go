//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/builder/importer"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type importComponentImpl struct {
	repoStore         database.RepoStore
	userStore         database.UserStore
	importer          importer.Importer
	mirrorSourceStore database.MirrorSourceStore
}

// NewImportComponent returns the CE import component stub.
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
