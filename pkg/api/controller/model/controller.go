package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

type Controller struct {
	modelStore database.ModelStore
	modelCache cache.ModelCache
}

func NewController(modelStore database.ModelStore, modelCache cache.ModelCache) *Controller {
	return &Controller{
		modelStore: modelStore,
		modelCache: modelCache,
	}
}
