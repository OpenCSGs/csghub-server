package accesstoken

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Create(ctx *gin.Context) (token *database.AccessToken, err error) {
	username := ctx.Param("username")
	var req types.CreateUserTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}
	req.Username = username

	token, err = c.gitServer.CreateUserToken(&req)
	if err == nil {
		user, err := c.userStore.FindByUsername(ctx, username)
		if err != nil {
			return nil, err
		}
		token.UserID = user.ID
		err = c.accessTokenStore.Create(ctx, token)
		if err != nil {
			return nil, err
		}
		token, err = c.accessTokenStore.FindByID(ctx, token.ID)
		if err != nil {
			return nil, err
		}
	}
	return
}
