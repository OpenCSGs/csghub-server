package mirror

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mirror/reposyncer"
)

type RepoSyncWorker interface {
	Run()
	SyncRepo(ctx context.Context, mirror *database.Mirror, mt *database.MirrorTask) (*database.MirrorTask, error)
}

func NewRepoSyncWorker(config *config.Config, numWorkers int) (RepoSyncWorker, error) {
	return reposyncer.NewRepoSyncWorker(config, numWorkers)
}
