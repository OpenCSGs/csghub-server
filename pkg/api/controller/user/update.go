package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/store/database"
	"opencsg.com/starhub-server/pkg/types"
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
