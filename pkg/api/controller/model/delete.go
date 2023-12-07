package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Delete(ctx *gin.Context) (err error) {
	username, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return err
	}
	err = c.gitServer.DeleteModelRepo(username, name)
	if err == nil {
		err = c.modelStore.Delete(ctx, username, name)
		if err != nil {
			return
		}
	}
	return
}
