package dataset

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
)

func (c *Controller) Tags(ctx *gin.Context) (tags []*types.DatasetTag, err error) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		return
	}

	tags, err = c.gitServer.GetDatasetTags(namespace, name, per, page)
	if err != nil {
		return
	}
	return
}
