package dataset

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Tags(ctx *gin.Context) ([]*types.DatasetTag, error) {
	return []*types.DatasetTag{
		{
			Name:    "tag1",
			Message: "Commit message",
			Commit: types.DatasetTagCommit{
				ID: "94991886af3e3820aa09fa353b29cf8557c93168",
			},
		},
		{
			Name:    "tag2",
			Message: "Commit message",
			Commit: types.DatasetTagCommit{
				ID: "94991886af3e3820aa09fa353b29cf8557c93168",
			},
		},
	}, nil
}
