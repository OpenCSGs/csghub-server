package model

import (
	"errors"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/pkg/types"
	"opencsg.com/starhub-server/pkg/utils/common"
)

func (c *Controller) Commits(ctx *gin.Context) (commits []*types.Commit, err error) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		return
	}
	ref := ctx.Query("ref")
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
	commits, err = c.gitServer.GetModelCommits(namespace, name, ref, per, page)
	if err != nil {
		return
	}
	return
}
