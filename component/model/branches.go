package model

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
)

func (c *Controller) Branches(ctx *gin.Context) (branches []*types.ModelBranch, err error) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		return
	}
	branches, err = c.gitServer.GetModelBranches(namespace, name, per, page)
	if err != nil {
		return
	}
	return
}
