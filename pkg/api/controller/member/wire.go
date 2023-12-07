package member

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/gitserver"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

func ProvideController(
	memberStore *database.MemberStore,
	memberCache *cache.MemberCache,
	orgStore *database.OrgStore,
	orgCache *cache.OrgCache,
	gitServer gitserver.GitServer,
) *Controller {
	return NewController(memberStore, memberCache, orgStore, orgCache, gitServer)
}
