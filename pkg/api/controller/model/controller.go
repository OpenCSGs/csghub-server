package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/gitserver"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

type Controller struct {
	modelStore     *database.ModelStore
	modelCache     *cache.ModelCache
	userStore      *database.UserStore
	userCache      *cache.UserCache
	orgStore       *database.OrgStore
	orgCache       *cache.OrgCache
	namespaceStore *database.NamespaceStore
	namespaceCache *cache.NamespaceCache
	repoStore      *database.RepoStore
	repoCache      *cache.RepoCache
	gitServer      gitserver.GitServer
}

func NewController(
	modelStore *database.ModelStore,
	modelCache *cache.ModelCache,
	userStore *database.UserStore,
	userCache *cache.UserCache,
	orgStore *database.OrgStore,
	orgCache *cache.OrgCache,
	namespaceStore *database.NamespaceStore,
	namespaceCache *cache.NamespaceCache,
	repoStore *database.RepoStore,
	repoCache *cache.RepoCache,
	gitServer gitserver.GitServer,
) *Controller {
	return &Controller{
		modelStore:     modelStore,
		modelCache:     modelCache,
		userStore:      userStore,
		userCache:      userCache,
		orgStore:       orgStore,
		orgCache:       orgCache,
		namespaceStore: namespaceStore,
		namespaceCache: namespaceCache,
		repoStore:      repoStore,
		repoCache:      repoCache,
		gitServer:      gitServer,
	}
}
