package accesstoken

import (
	"errors"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/types"
)

func (c *Controller) Delete(ctx *gin.Context) (err error) {
	username := ctx.Param("username")
	userExists, err := c.userStore.IsExist(ctx, username)
	if err != nil {
		return
	}
	if !userExists {
		err = errors.New("User does not exist.")
		return
	}
	tokenName := ctx.Param("token_name")
	tkExists, err := c.accessTokenStore.IsExist(ctx, username, tokenName)
	if err != nil {
		return
	}
	if !tkExists {
		err = errors.New("Token does not exist.")
		return
	}
	err = c.gitServer.DeleteUserToken(&types.DeleteUserTokenRequest{
		Username: username,
		Name:     tokenName,
	})
	if err != nil {
		return
	}
	err = c.accessTokenStore.Delete(ctx, username, tokenName)
	if err != nil {
		return
	}

	return
}
