//go:build !ee && !saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/types"
)

// createAdminRoutes has no sensitive word administration endpoints in this edition.
func createAdminRoutes(_ *gin.RouterGroup, _ func(...types.AdminPermission) gin.HandlerFunc) {}
