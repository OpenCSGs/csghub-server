//go:build !ee && !saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/user/handler"
)

// enableUnit reports whether hierarchy organization routes are enabled in this edition.
func enableUnit(_ *config.Config) bool {
	return false
}

func extendRoutes(_ *gin.RouterGroup, _ middleware.MiddlewareCollection, _ *config.Config, _ *handler.UserHandler) error {
	return nil
}
