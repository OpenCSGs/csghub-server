package dataset

import (
	"errors"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Update(ctx *gin.Context) (dataset *types.Dataset, err error) {
	var req types.UpdateDatasetReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}
	repo, err := c.datasetStore.FindyByRepoPath(ctx, namespace, name)
	if err != nil {
		return
	}

	if repo == nil {
		return nil, errors.New("The repository with given path and name is not found")
	}

	dataset, err = c.gitServer.UpdateDatasetRepo(namespace, name, repo, &req)
	if err == nil {
		err = c.datasetStore.UpdateRepo(ctx, dataset)
		if err != nil {
			return
		}
	}
	return
}
