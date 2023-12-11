package dataset

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/utils/common"
)

func (c *Controller) Index(ctx *gin.Context) (datasets []database.Dataset, total int, err error) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		return
	}
	datasets, err = c.datasetStore.Public(ctx, per, page)
	if err != nil {
		return
	}
	total, err = c.datasetStore.PublicCount(ctx)
	if err != nil {
		return
	}
	return
}
