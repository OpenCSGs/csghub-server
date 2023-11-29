package dataset

import (
	"errors"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
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
		repo, err := c.datasetStore.FindyByRepoPath(ctx, namespace, name)
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, errors.New("The repository with given path and name is not found")
		}
		ref = repo.DefaultBranch
	}

	tree, err = c.gitServer.GetModelFileTree(namespace, name, ref, path)
	return
}
