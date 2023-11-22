package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

func ProvideController(modelStore database.ModelStore, modelCache cache.ModelCache) *Controller {
	return NewController(modelStore, modelCache)
}
