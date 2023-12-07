package user

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
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
