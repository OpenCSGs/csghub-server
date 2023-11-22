package dataset

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Detail(ctx *gin.Context) (*types.DatasetDetail, error) {
	return &types.DatasetDetail{
		Path:          "01ai",
		Name:          "Yi-6B-200K",
		Introduction:  "## Introduction...",
		License:       "license",
		DownloadCount: 100,
		LastUpdatedAt: "2023-10-10 10:10:10",
	}, nil
}
