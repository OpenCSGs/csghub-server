package model

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/gin-gonic/gin"
)

func (c *Controller) Create(ctx *gin.Context) (*types.Model, error) {
	return &types.Model{
		UserID:    "test123",
		Namespace: "namespace",
		Name:      "name",
		Public:    true,
	}, nil
}
