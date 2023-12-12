package accesstoken

import (
	"fmt"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
)

type Controller struct {
	userStore        *database.UserStore
	accessTokenStore *database.AccessTokenStore
	gitServer        gitserver.GitServer
}

func New(config *config.Config) (*Controller, error) {
	gs, err := gitserver.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitserver:%w", err)
	}
	return &Controller{
		userStore:        database.NewUserStore(),
		accessTokenStore: database.NewAccessTokenStore(),
		gitServer:        gs,
	}, nil
}
