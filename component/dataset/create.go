package dataset

import (
	"errors"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/types"
)

func (c *Controller) Create(ctx *gin.Context) (dataset *database.Dataset, err error) {
	var req types.CreateDatasetReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	_, err = c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("Namespace does not exist")
	}

	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("User does not exist")
	}

	dataset, repo, err := c.gitServer.CreateDatasetRepo(&req)
	if err == nil {
		err = c.datasetStore.Create(ctx, dataset, repo, user.ID)
		if err != nil {
			return
		}
	}
	return
}
