package accesstoken

import (
	"opencsg.com/starhub-server/pkg/gitserver"
	"opencsg.com/starhub-server/pkg/store/cache"
	"opencsg.com/starhub-server/pkg/store/database"
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
