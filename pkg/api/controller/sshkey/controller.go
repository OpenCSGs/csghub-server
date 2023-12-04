package sshkey

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/gitserver"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
)

type Controller struct {
	sshKeyStore *database.SSHKeyStore
	sshKeyCache *cache.SSHKeyCache
	userStore   *database.UserStore
	gitServer   gitserver.GitServer
}

func NewController(
	sshKeyStore *database.SSHKeyStore,
	sshKeyCache *cache.SSHKeyCache,
	userStore *database.UserStore,
	gitServer gitserver.GitServer,
) *Controller {
	return &Controller{
		sshKeyStore: sshKeyStore,
		sshKeyCache: sshKeyCache,
		userStore:   userStore,
		gitServer:   gitServer,
	}
}
