//go:build !ee && !saas

package filter

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

type RepoFilter struct {
	repoStore   database.RepoStore
	mirrorStore database.MirrorStore
	cfg         *config.Config
}

func NewRepoFilter(cfg *config.Config) *RepoFilter {
	return &RepoFilter{
		repoStore:   database.NewRepoStore(),
		mirrorStore: database.NewMirrorStore(),
		cfg:         cfg,
	}
}

func (rf *RepoFilter) ShouldSync(ctx context.Context, repoID int64) (bool, string, error) {
	return true, "", nil
}
