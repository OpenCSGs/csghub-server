package user

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/store/database"
	"opencsg.com/starhub-server/pkg/types"
)

func (c *Controller) Create(ctx *gin.Context) (*database.User, error) {
	var req types.CreateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	respUser, err := c.gitServer.CreateUser(&req)
	if err == nil {
		namespace := &database.Namespace{
			Path: respUser.Username,
		}
		c.userStore.Create(ctx, respUser, namespace)
	}

	return respUser, err
}
