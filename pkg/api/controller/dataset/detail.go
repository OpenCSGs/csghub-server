package dataset

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Detail(ctx *gin.Context) (detail *types.DatasetDetail, err error) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}
	detail, err = c.gitServer.GetDatasetDetail(namespace, name)
	if err != nil {
		return
	}
	return
}
