package sshkey

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/gitserver"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

func ProvideController(
	sshKeyStore *database.SSHKeyStore,
	sshKeyCache *cache.SSHKeyCache,
	userStore *database.UserStore,
	gitServer gitserver.GitServer,
) *Controller {
	return NewController(sshKeyStore, sshKeyCache, userStore, gitServer)
}
