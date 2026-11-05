//go:build !ee && !saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func registerSecurityScanRoutes(_ *gin.RouterGroup, _ *gin.RouterGroup, _ *config.Config, _ gin.HandlerFunc, _ func(...types.AdminPermission) gin.HandlerFunc) error {
	return nil
}
