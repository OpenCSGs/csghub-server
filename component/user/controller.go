package user

import (
	"fmt"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
)

type Controller struct {
	userStore    *database.UserStore
	modelStore   *database.ModelStore
	datasetStore *database.DatasetStore
	gitServer    gitserver.GitServer
}

func New(config *config.Config) (*Controller, error) {
	gs, err := gitserver.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitserver:%w", err)
	}
	return &Controller{
		datasetStore: database.NewDatasetStore(),
		userStore:    database.NewUserStore(),
		gitServer:    gs,
	}, nil
}
