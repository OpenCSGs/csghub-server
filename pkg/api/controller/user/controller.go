package user

import (
	"opencsg.com/starhub-server/pkg/gitserver"
	"opencsg.com/starhub-server/pkg/store/cache"
	"opencsg.com/starhub-server/pkg/store/database"
)

type Controller struct {
	userStore      *database.UserStore
	userCache      *cache.UserCache
	modelStore     *database.ModelStore
	modelCache     *cache.ModelCache
	datasetStore   *database.DatasetStore
	datasetCache   *cache.DatasetCache
	namespaceStore *database.NamespaceStore
	namespaceCache *cache.NamespaceCache
	gitServer      gitserver.GitServer
}

func NewController(
	userStore *database.UserStore,
	userCache *cache.UserCache,
	modelStore *database.ModelStore,
	modelCache *cache.ModelCache,
	datasetStore *database.DatasetStore,
	datasetCache *cache.DatasetCache,
	namespaceStore *database.NamespaceStore,
	namespaceCache *cache.NamespaceCache,
	gitServer gitserver.GitServer,
) *Controller {
	return &Controller{
		userStore:      userStore,
		userCache:      userCache,
		modelStore:     modelStore,
		modelCache:     modelCache,
		datasetStore:   datasetStore,
		datasetCache:   datasetCache,
		namespaceStore: namespaceStore,
		namespaceCache: namespaceCache,
		gitServer:      gitServer,
	}
}
