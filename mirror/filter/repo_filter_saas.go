//go:build saas || ee

package filter

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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
	var message string
	repo, err := rf.repoStore.FindById(ctx, repoID)
	if err != nil {
		return false, message, fmt.Errorf("failed to find repo by id: %w", err)
	}

	mirror, err := rf.mirrorStore.FindByRepoID(ctx, repoID)
	if err != nil {
		return false, message, fmt.Errorf("failed to find mirror by repo id: %w", err)
	}

	if repo.RepositoryType == types.DatasetRepo && repo.LFSObjectsSize > rf.cfg.Mirror.MaxDatasetRepoSize {
		message = fmt.Sprintf("dataset repo size exceeds the maximum allowed size of %d bytes", rf.cfg.Mirror.MaxDatasetRepoSize)
		return false, message, nil
	}

	if mirror.Priority == types.P3MirrorPriority && repo.LFSObjectsSize > rf.cfg.Mirror.MaxModelRepoSize {
		message = fmt.Sprintf("model repo size exceeds the maximum allowed size of %d bytes", rf.cfg.Mirror.MaxModelRepoSize)
		return false, message, nil
	}

	return true, message, nil
}
