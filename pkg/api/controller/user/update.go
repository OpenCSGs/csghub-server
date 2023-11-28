package user

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Update(ctx *gin.Context) (*database.User, error) {
	var req types.UpdateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	respCode, respUser, err := c.gitServer.UpdateUser(&req)
	if err == nil && respCode == http.StatusOK {
		c.userStore.UpdateByUsername(ctx, respUser)
	}

	return respUser, err
}
