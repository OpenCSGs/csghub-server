package model

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/utils/common"
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
