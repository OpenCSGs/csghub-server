//go:build !saas && !ee

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/runner/handler"
)

func addClusterNodeRoutes(clusterGroup *gin.RouterGroup, clusterHandler *handler.ClusterHandler) {
	// No-op for CE
}
