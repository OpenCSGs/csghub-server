package model

import (
	"fmt"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
)

type Controller struct {
	modelStore     *database.ModelStore
	userStore      *database.UserStore
	namespaceStore *database.NamespaceStore
	gitServer      gitserver.GitServer
}

func New(config *config.Config) (*Controller, error) {
	gs, err := gitserver.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitserver:%w", err)
	}
	return &Controller{
		modelStore:     database.NewModelStore(),
		userStore:      database.NewUserStore(),
		namespaceStore: database.NewNamespaceStore(),
		gitServer:      gs,
	}, nil
}

func NewController(
	modelStore *database.ModelStore,
	userStore *database.UserStore,
	orgStore *database.OrgStore,
	namespaceStore *database.NamespaceStore,
	gitServer gitserver.GitServer,
) *Controller {
	return &Controller{
		modelStore:     modelStore,
		userStore:      userStore,
		namespaceStore: namespaceStore,
		gitServer:      gitServer,
	}
}
