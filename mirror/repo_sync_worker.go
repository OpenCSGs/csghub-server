package mirror

import (
	"context"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mirror/queue"
	"opencsg.com/csghub-server/mirror/reposyncer"
)

type RepoSyncWorker interface {
	Run()
	SyncRepo(ctx context.Context, task queue.MirrorTask) error
}

func NewRepoSyncWorker(config *config.Config, numWorkers int) (RepoSyncWorker, error) {
	return reposyncer.NewLocalMirrorWoker(config, numWorkers)
}
