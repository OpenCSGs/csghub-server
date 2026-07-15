package component

import (
	"context"
	"errors"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// ErrInvalidPath reports an invalid GitLab import path.
var ErrInvalidPath = errors.New("invalid path")

// ErrRepoAlreadyExists reports an import target repository conflict.
var ErrRepoAlreadyExists = errors.New("repository already exists")

// ImportComponent defines repository import operations shared by all editions.
type ImportComponent interface {
	Import(ctx context.Context, req types.ImportReq) error
	GetGitlabRepos(ctx context.Context, req *types.GetGitlabReposReq) ([]types.RemoteRepository, error)
	ImportStatus(ctx context.Context, req types.ImportStatusReq) ([]types.ImportedRepository, error)
}

// NewImportComponentImpl keeps the legacy import component constructor available across editions.
func NewImportComponentImpl(config *config.Config) (ImportComponent, error) {
	return NewImportComponent(config)
}
