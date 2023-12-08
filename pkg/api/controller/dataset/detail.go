package dataset

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/types"
	"opencsg.com/starhub-server/pkg/utils/common"
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
