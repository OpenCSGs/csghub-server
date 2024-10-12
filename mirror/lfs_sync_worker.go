package mirror

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mirror/lfssyncer"
)

type LFSSyncWorker interface {
	Run()
	SyncLfs(ctx context.Context, workerID int, mirror *database.Mirror) error
}

func NewLFSSyncWorker(config *config.Config, numWorkers int) (LFSSyncWorker, error) {
	return lfssyncer.NewMinioLFSSyncWorker(config, numWorkers)
}
