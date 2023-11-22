package dataset

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

type Controller struct {
	datasetStore database.DatasetStore
	datasetCache cache.DatasetCache
}

func NewController(datasetStore database.DatasetStore, datasetCache cache.DatasetCache) *Controller {
	return &Controller{
		datasetStore: datasetStore,
		datasetCache: datasetCache,
	}
}
