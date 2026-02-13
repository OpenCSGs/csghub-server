//go:build !ee && !saas

package handler

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
)

func (h *ClusterHandler) GetAllNodes(ctx *gin.Context) {
	httpbase.OK(ctx, nil)
}

func (h *ClusterHandler) GetNodeByID(ctx *gin.Context) {
	httpbase.OK(ctx, nil)
}

func (h *ClusterHandler) QueryClusterDeploys(ctx *gin.Context) {
	httpbase.OK(ctx, nil)
}

func (h *ClusterHandler) QueryClusterWorkflows(ctx *gin.Context) {
	httpbase.OK(ctx, nil)
}
