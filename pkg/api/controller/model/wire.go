package model

import (
	"opencsg.com/starhub-server/pkg/gitserver"
	"opencsg.com/starhub-server/pkg/store/cache"
	"opencsg.com/starhub-server/pkg/store/database"
)

func ProvideController(
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
	return NewController(modelStore, modelCache, userStore, userCache, orgStore, orgCache,
		namespaceStore, namespaceCache, repoStore, repoCache, gitServer)
}
