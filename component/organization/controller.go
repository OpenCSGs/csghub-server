package organization

import (
	"fmt"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
)

type Controller struct {
	orgStore       *database.OrgStore
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
		orgStore:       database.NewOrgStore(),
		userStore:      database.NewUserStore(),
		namespaceStore: database.NewNamespaceStore(),
		gitServer:      gs,
	}, nil
}
