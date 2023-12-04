package user

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/gitserver"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

type Controller struct {
	userStore    *database.UserStore
	userCache    *cache.UserCache
	modelStore   *database.ModelStore
	modelCache   *cache.ModelCache
	datasetStore *database.DatasetStore
	datasetCache *cache.DatasetCache
	gitServer    gitserver.GitServer
}

func NewController(
	userStore *database.UserStore,
	userCache *cache.UserCache,
	modelStore *database.ModelStore,
	modelCache *cache.ModelCache,
	datasetStore *database.DatasetStore,
	datasetCache *cache.DatasetCache,
	gitServer gitserver.GitServer,
) *Controller {
	return &Controller{
		userStore:    userStore,
		userCache:    userCache,
		modelStore:   modelStore,
		modelCache:   modelCache,
		datasetStore: datasetStore,
		datasetCache: datasetCache,
		gitServer:    gitServer,
	}
}
