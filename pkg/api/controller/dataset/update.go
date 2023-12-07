package dataset

import (
	"errors"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Update(ctx *gin.Context) (dataset *database.Dataset, err error) {
	var req types.UpdateDatasetReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}

	_, err = c.namespaceStore.FindByPath(ctx, namespace)
	if err != nil {
		return nil, errors.New("Namespace does not exist")
	}

	_, err = c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("User does not exist")
	}

	dataset, err = c.datasetStore.FindyByPath(ctx, namespace, name)
	if err != nil {
		return
	}

	err = c.gitServer.UpdateDatasetRepo(namespace, name, dataset, dataset.Repository, &req)
	if err == nil {
		err = c.datasetStore.Update(ctx, dataset, dataset.Repository)
		if err != nil {
			return
		}
	}
	return
}
