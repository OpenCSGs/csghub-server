package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/gitserver"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

func ProvideController(
	modelStore *database.ModelStore,
	modelCache *cache.ModelCache,
	gitServer gitserver.GitServer,
) *Controller {
	return NewController(modelStore, modelCache, gitServer)
}
