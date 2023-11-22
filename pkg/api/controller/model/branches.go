package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Branches(ctx *gin.Context) ([]*types.ModelBranch, error) {
	return []*types.ModelBranch{
		{
			Name:    "branch1",
			Message: "Commit message",
			Commit: types.ModelBranchCommit{
				ID: "94991886af3e3820aa09fa353b29cf8557c93168",
			},
		},
		{
			Name:    "branch2",
			Message: "Commit message",
			Commit: types.ModelBranchCommit{
				ID: "94991886af3e3820aa09fa353b29cf8557c93168",
			},
		},
	}, nil
}
