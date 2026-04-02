//go:build !saas && !ee

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/runner/handler"
)

func addClusterNodeRoutes(clusterGroup *gin.RouterGroup, clusterHandler *handler.ClusterHandler) {
	// No-op for CE
}

func addSandboxRoutes(apiGroup *gin.RouterGroup, config *config.Config, clusterPool cluster.Pool) error {
	// No-op for CE
	return nil
}
