package model

import (
	"errors"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) FileRaw(ctx *gin.Context) (raw string, err error) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}
	path := ctx.Param("file_path")
	ref := ctx.Query("ref")
	if ref == "" {
		model, err := c.modelStore.FindyByPath(ctx, namespace, name)
		if err != nil {
			return "", err
		}
		if model == nil {
			err = errors.New("The repository with given path and name is not found")
			return "", err
		}
		ref = model.Repository.DefaultBranch
	}
	raw, err = c.gitServer.GetModelFileRaw(namespace, name, ref, path)
	if err != nil {
		return
	}
	return
}
