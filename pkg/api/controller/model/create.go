package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Create(ctx *gin.Context) (model *types.Model, err error) {
	var req types.CreateModelReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	model, err = c.gitServer.CreateModelRepo(&req)
	if err == nil {
		err = c.modelStore.CreateRepo(ctx, database.Repository(*model))
		if err != nil {
			return
		}
	}
	return
}
