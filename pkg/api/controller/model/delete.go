package model

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/utils/common"
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
