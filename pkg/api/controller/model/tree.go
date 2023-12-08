package model

import (
	"errors"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/types"
	"opencsg.com/starhub-server/pkg/utils/common"
)

func (c *Controller) Tree(ctx *gin.Context) (tree []*types.File, err error) {
	// TODO: Add parameter validation
	var ref string

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}
	path := ctx.Query("path")
	ref = ctx.Query("ref")
	if ref == "" {
		model, err := c.modelStore.FindyByPath(ctx, namespace, name)
		if err != nil {
			return nil, err
		}
		if model == nil {
			return nil, errors.New("The repository with given path and name is not found")
		}
		ref = model.Repository.DefaultBranch
	}

	tree, err = c.gitServer.GetModelFileTree(namespace, name, ref, path)
	return
}
