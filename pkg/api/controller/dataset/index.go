package dataset

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Index(ctx *gin.Context) (datasets []types.Dataset, total int, err error) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		return
	}
	datasets, err = c.datasetStore.PublicRepos(ctx, per, page)
	if err != nil {
		return
	}
	total, err = c.datasetStore.PublicRepoCount(ctx)
	if err != nil {
		return
	}
	return
}
