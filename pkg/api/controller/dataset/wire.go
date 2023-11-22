package dataset

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

func ProvideController(datasetStore database.DatasetStore, datasetCache cache.DatasetCache) *Controller {
	return NewController(datasetStore, datasetCache)
}
