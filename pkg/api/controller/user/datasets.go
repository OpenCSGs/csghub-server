package user

import (
	"errors"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Datasets(ctx *gin.Context) (datasets []types.Dataset, err error) {
	currentUser := ctx.Query("current_user")
	username := ctx.Param("username")
	_, err = c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return nil, errors.New("User does not exist")
	}

	_, err = c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, errors.New("Current user does not exist")
	}

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		return nil, err
	}

	onlyPublic := currentUser != username

	datasets, err = c.datasetStore.RepoByUsername(ctx, username, per, page, onlyPublic)
	if err != nil {
		return nil, err
	}

	return
}
