package model

import (
	"errors"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/types"
)

func (c *Controller) Create(ctx *gin.Context) (model *database.Model, err error) {
	var req types.CreateModelReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	_, err = c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("Namespace does not exist")
	}

	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("User does not exist")
	}

	model, repo, err := c.gitServer.CreateModelRepo(&req)
	if err == nil {
		err = c.modelStore.Create(ctx, model, repo, user.ID)
		if err != nil {
			return
		}
	}
	return
}
