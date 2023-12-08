package organization

import (
	"opencsg.com/starhub-server/pkg/gitserver"
	"opencsg.com/starhub-server/pkg/store/cache"
	"opencsg.com/starhub-server/pkg/store/database"
)

type Controller struct {
	memberStore    *database.MemberStore
	memberCache    *cache.MemberCache
	orgStore       *database.OrgStore
	orgCache       *cache.OrgCache
	userStore      *database.UserStore
	userCache      *cache.UserCache
	namespaceStore *database.NamespaceStore
	namespaceCache *cache.NamespaceCache
	gitServer      gitserver.GitServer
}

func NewController(
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
	return &Controller{
		memberStore:    memberStore,
		memberCache:    memberCache,
		orgStore:       orgStore,
		orgCache:       orgCache,
		userStore:      userStore,
		userCache:      userCache,
		namespaceStore: namespaceStore,
		namespaceCache: namespaceCache,
		gitServer:      gitServer,
	}
}
