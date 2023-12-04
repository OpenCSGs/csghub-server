package dataset

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/gitserver"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

func ProvideController(
	datasetStore *database.DatasetStore,
	datasetCache *cache.DatasetCache,
	userStore *database.UserStore,
	userCache *cache.UserCache,
	gitServer gitserver.GitServer,
) *Controller {
	return NewController(datasetStore, datasetCache, userStore, userCache, gitServer)
}
