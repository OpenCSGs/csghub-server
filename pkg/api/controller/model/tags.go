package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Tags(ctx *gin.Context) (tags []*types.ModelTag, err error) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		return
	}

	tags, err = c.gitServer.GetModelTags(namespace, name, per, page)
	if err != nil {
		return
	}
	return
}
