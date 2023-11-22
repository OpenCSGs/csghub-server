package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Tree(ctx *gin.Context) ([]*types.File, error) {
	return []*types.File{
		{
			Name: "test.rb",
			Type: "dir",
			Lfs:  false,
			Commit: types.Commit{
				ID:             "94991886af3e3820aa09fa353b29cf8557c93168",
				CommitterName:  "vincent",
				CommitterEmail: "vincent@gmail.com",
				CommitterDate:  "2023-10-10 10:10:10",
				CreatedAt:      "2023-10-10 10:10:10",
				Title:          "Add some files",
				Message:        "Add some files",
				AuthorName:     "vincent",
				AuthorEmail:    "vincent@gmail.com",
				AuthoredDate:   "2023-10-10 10:10:10",
			},
		},
		{
			Name: "commit.json",
			Type: "file",
			Size: 99999999,
			Lfs:  true,
			Commit: types.Commit{
				ID:             "94991886af3e3820aa09fa353b29cf8557c93168",
				CommitterName:  "vincent",
				CommitterEmail: "vincent@gmail.com",
				CommitterDate:  "2023-10-10 10:10:10",
				CreatedAt:      "2023-10-10 10:10:10",
				Title:          "Add some files",
				Message:        "Add some files",
				AuthorName:     "vincent",
				AuthorEmail:    "vincent@gmail.com",
				AuthoredDate:   "2023-10-10 10:10:10",
			},
		},
	}, nil
}
