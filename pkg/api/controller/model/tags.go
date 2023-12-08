package model

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/types"
	"opencsg.com/starhub-server/pkg/utils/common"
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
