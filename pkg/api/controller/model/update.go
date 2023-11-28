package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Update(ctx *gin.Context) (model *types.Model, err error) {
	// var req types.UpdateModelReq
	// if err := ctx.ShouldBindJSON(&req); err != nil {
	// 	return nil, err
	// }

	// model, err = c.gitServer.UpdateModelRepo(&req)
	// if err == nil {
	// 	err = c.modelStore.CreateRepo(ctx, database.Repository(*model))
	// 	if err != nil {
	// 		return
	// 	}
	// }
	return
}
