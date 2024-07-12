package gitea

import (
	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *Client) CreateUserToken(req *types.CreateUserTokenRequest) (token *database.AccessToken, err error) {
	giteaToken, _, err := c.giteaClient.CreateAccessToken(
		gitea.CreateAccessTokenOption{
			Username: req.Username,
			Name:     req.TokenName,
			Scopes:   []gitea.AccessTokenScope{"write:repository"},
		},
	)

	if err != nil {
		return
	}

	token = &database.AccessToken{
		GitID: giteaToken.ID,
		Name:  req.TokenName,
		Token: giteaToken.Token,
	}

	return
}

func (c *Client) DeleteUserToken(req *types.DeleteUserTokenRequest) (err error) {
	_, err = c.giteaClient.DeleteAccessToken(gitea.DeleteAccessTokenOption{
		Username: req.Username,
		Value:    req.TokenName,
	})
	return
}
