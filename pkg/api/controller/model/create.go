package model

import (
	"errors"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Create(ctx *gin.Context) (model *types.Model, err error) {
	var req types.CreateModelReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("User does not exist")
	}

	model, err = c.gitServer.CreateModelRepo(&req)
	if err == nil {
		err = c.modelStore.CreateRepo(ctx, model, user.ID)
		if err != nil {
			return
		}
	}
	return
}
