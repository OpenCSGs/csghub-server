package user

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/gitserver"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

func ProvideController(
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
	return NewController(userStore, userCache, modelStore, modelCache, datasetStore, datasetCache,
		namespaceStore, namespaceCache, gitServer)
}
