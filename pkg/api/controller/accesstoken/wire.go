package accesstoken

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/gitserver"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

func ProvideController(
	userStore *database.UserStore,
	userCache *cache.UserCache,
	accesstokenStore *database.AccessTokenStore,
	accesstokenCache *cache.AccessTokenCache,
	gitServer gitserver.GitServer,
) *Controller {
	return NewController(userStore, userCache, accesstokenStore, accesstokenCache, gitServer)
}
