package dataset

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
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
