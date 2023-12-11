package dataset

import (
	"errors"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
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
		dataset, err := c.datasetStore.FindyByPath(ctx, namespace, name)
		if err != nil {
			return nil, err
		}
		if dataset == nil {
			return nil, errors.New("The repository with given path and name is not found")
		}
		ref = dataset.Repository.DefaultBranch
	}

	tree, err = c.gitServer.GetDatasetFileTree(namespace, name, ref, path)
	return
}
