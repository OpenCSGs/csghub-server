package gitea

import (
	"github.com/pulltheflower/gitea-go-sdk/gitea"
	"opencsg.com/starhub-server/pkg/store/database"
	"opencsg.com/starhub-server/pkg/types"
)

func (c *Client) CreateUserToken(req *types.CreateUserTokenRequest) (token *database.AccessToken, err error) {
	giteaToken, _, err := c.giteaClient.CreateAccessToken(
		gitea.CreateAccessTokenOption{
			Username: req.Username,
			Name:     req.Name,
			Scopes:   []gitea.AccessTokenScope{"write:repository"},
		},
	)

	if err != nil {
		return
	}

	token = &database.AccessToken{
		GitID: giteaToken.ID,
		Name:  req.Name,
		Token: giteaToken.Token,
	}

	return
}

func (c *Client) DeleteUserToken(req *types.DeleteUserTokenRequest) (err error) {
	_, err = c.giteaClient.DeleteAccessToken(gitea.DeleteAccessTokenOption{
		Username: req.Username,
		Value:    req.Name,
	})
	return
}
