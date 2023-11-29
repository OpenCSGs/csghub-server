package model

import (
	"errors"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/utils/common"
	"github.com/gin-gonic/gin"
)

func (c *Controller) LastCommit(ctx *gin.Context) (commit *types.Commit, err error) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		return
	}
	ref := ctx.Query("ref")
	if ref == "" {
		repo, err := c.modelStore.FindyByRepoPath(ctx, namespace, name)
		if err != nil {
			return nil, err
		}
		if repo == nil {
			return nil, errors.New("The repository with given path and name is not found")
		}
		ref = repo.DefaultBranch
	}
	commit, err = c.gitServer.GetModelLastCommit(namespace, name, ref)
	if err != nil {
		return
	}
	return
}
