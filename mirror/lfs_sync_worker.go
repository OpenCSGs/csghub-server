package mirror

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/mirror/lfssyncer"
)

type LFSSyncWorker interface {
	SetContext(ctx context.Context)
	Run(mt *database.MirrorTask)
}

func NewLFSSyncWorker(config *config.Config, id int) (LFSSyncWorker, error) {
	return lfssyncer.NewLfsSyncWorker(config, id)

}
