//go:build !ee && !saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/user/handler"
)

func extendRoutes(_ *gin.RouterGroup, _ middleware.MiddlewareCollection, _ *config.Config, _ *handler.UserHandler) error {
	return nil
}
