package dataset

import (
	"errors"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Create(ctx *gin.Context) (dataset *types.Dataset, err error) {
	var req types.CreateDatasetReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		return nil, err
	}

	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("User does not exist")
	}

	dataset, err = c.gitServer.CreateDatasetRepo(&req)
	if err == nil {
		err = c.datasetStore.CreateRepo(ctx, dataset, user.ID)
		if err != nil {
			return
		}
	}
	return
}
