package sshkey

import (
	"opencsg.com/starhub-server/pkg/gitserver"
	"opencsg.com/starhub-server/pkg/store/cache"
	"opencsg.com/starhub-server/pkg/store/database"
)

func ProvideController(
	sshKeyStore *database.SSHKeyStore,
	sshKeyCache *cache.SSHKeyCache,
	userStore *database.UserStore,
	gitServer gitserver.GitServer,
) *Controller {
	return NewController(sshKeyStore, sshKeyCache, userStore, gitServer)
}
