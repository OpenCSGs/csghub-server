package gitea

import (
	"context"

	"github.com/pulltheflower/gitea-go-sdk/gitea"
	"opencsg.com/starhub-server/config"
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
