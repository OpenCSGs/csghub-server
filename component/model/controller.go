package model

import (
	"context"
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
	dbConfig := database.DBConfig{
		Dialect: database.DatabaseDialect(config.Database.Driver),
		DSN:     config.Database.DSN,
	}

	db, err := database.NewDB(context.Background(), dbConfig)
	if err != nil {
		return nil, err
	}

	gs, err := gitserver.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitserver:%w", err)
	}
	return &Controller{
		modelStore:     database.NewModelStore(db),
		userStore:      database.NewUserStore(db),
		namespaceStore: database.NewNamespaceStore(db),
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
