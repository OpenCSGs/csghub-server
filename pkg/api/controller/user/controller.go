package user

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/gitserver"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

type Controller struct {
	userStore *database.UserStore
	userCache *cache.UserCache
	gitServer gitserver.GitServer
}

func NewController(
	userStore *database.UserStore,
	userCache *cache.UserCache,
	gitServer gitserver.GitServer,
) *Controller {
	return &Controller{
		userStore: userStore,
		userCache: userCache,
		gitServer: gitServer,
	}
}
