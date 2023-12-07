package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Index(ctx *gin.Context) (models []database.Model, total int, err error) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		return
	}
	models, err = c.modelStore.Public(ctx, per, page)
	if err != nil {
		return
	}
	total, err = c.modelStore.PublicCount(ctx)
	if err != nil {
		return
	}
	return
}
