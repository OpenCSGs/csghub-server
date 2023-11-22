package dataset

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Branches(ctx *gin.Context) ([]*types.DatasetBranch, error) {
	return []*types.DatasetBranch{
		{
			Name:    "branch1",
			Message: "Commit message",
			Commit: types.DatasetBranchCommit{
				ID: "94991886af3e3820aa09fa353b29cf8557c93168",
			},
		},
		{
			Name:    "branch2",
			Message: "Commit message",
			Commit: types.DatasetBranchCommit{
				ID: "94991886af3e3820aa09fa353b29cf8557c93168",
			},
		},
	}, nil
}
