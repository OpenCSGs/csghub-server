package gitea

import (
	"context"
	"log"

	"code.gitea.io/sdk/gitea"
	"git-devops.opencsg.com/product/community/starhub-server/config"
)

type Client struct {
	giteaClient *gitea.Client
}

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

const APIBaseUrl = "api/v1"

func NewClient(config *config.Config) (client *Client, err error) {
	ctx := context.Background()
	log.Println("================================")
	log.Print(config.GitServer.SecretKey)
	giteaClient, err := gitea.NewClient(
		config.GitServer.Host,
		gitea.SetContext(ctx),
		gitea.SetToken(config.GitServer.SecretKey),
	)

	if err != nil {
		return nil, err
	}

	return &Client{giteaClient: giteaClient}, nil
}
