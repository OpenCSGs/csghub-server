package dataset

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Branches(ctx *gin.Context) (branches []*types.DatasetBranch, err error) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		return
	}
	branches, err = c.gitServer.GetDatasetBranches(namespace, name, per, page)
	if err != nil {
		return
	}
	return
}
