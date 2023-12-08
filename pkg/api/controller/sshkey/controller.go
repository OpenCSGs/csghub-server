package sshkey

import (
	"opencsg.com/starhub-server/pkg/gitserver"
	"opencsg.com/starhub-server/pkg/store/cache"
	"opencsg.com/starhub-server/pkg/store/database"
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
