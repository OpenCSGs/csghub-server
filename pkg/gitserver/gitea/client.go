package gitea

import (
	"context"

	"git-devops.opencsg.com/product/community/starhub-server/config"
	"github.com/pulltheflower/gitea-go-sdk/gitea"
)

type Client struct {
	giteaClient *gitea.Client
}

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func NewClient(config *config.Config) (client *Client, err error) {
	ctx := context.Background()
	giteaClient, err := gitea.NewClient(
		config.GitServer.Host,
		gitea.SetContext(ctx),
		gitea.SetToken(config.GitServer.SecretKey),
		gitea.SetBasicAuth(config.GitServer.Username, config.GitServer.Password),
	)

	if err != nil {
		return nil, err
	}

	return &Client{giteaClient: giteaClient}, nil
}
