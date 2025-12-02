//go:build !ee && !saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

func extendRoutes(_ *gin.RouterGroup, _ middleware.MiddlewareCollection, _ *config.Config) error {
	return nil
}
