package organization

import (
	"opencsg.com/starhub-server/pkg/gitserver"
	"opencsg.com/starhub-server/pkg/store/cache"
	"opencsg.com/starhub-server/pkg/store/database"
)

func ProvideController(
	memberStore *database.MemberStore,
	memberCache *cache.MemberCache,
	orgStore *database.OrgStore,
	orgCache *cache.OrgCache,
	userStore *database.UserStore,
	userCache *cache.UserCache,
	namespaceStore *database.NamespaceStore,
	namespaceCache *cache.NamespaceCache,
	gitServer gitserver.GitServer,
) *Controller {
	return NewController(memberStore, memberCache, orgStore, orgCache, userStore, userCache,
		namespaceStore, namespaceCache, gitServer)
}
