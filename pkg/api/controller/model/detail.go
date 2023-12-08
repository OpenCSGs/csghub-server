package model

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/types"
	"opencsg.com/starhub-server/pkg/utils/common"
)

func (c *Controller) Detail(ctx *gin.Context) (detail *types.ModelDetail, err error) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}

	detail, err = c.gitServer.GetModelDetail(namespace, name)
	if err != nil {
		return
	}
	return
}
