package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Index(ctx *gin.Context) (models []types.Model, total int, err error) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		return
	}
	models, err = c.modelStore.PublicRepos(ctx, per, page)
	if err != nil {
		return
	}
	total, err = c.modelStore.PublicRepoCount(ctx)
	if err != nil {
		return
	}
	return
}
