//go:build !ee && !saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/user/handler"
)

func extendRoutes(api *gin.RouterGroup, _ middleware.MiddlewareCollection, _ *config.Config, h *handler.UserHandler) error {
	api.DELETE("/user/:username/close_account", h.CloseAccount)
	return nil
}

// createAdminBillingOrganizationRoutes leaves billing organization selectors unavailable in this edition.
func createAdminBillingOrganizationRoutes(_ *gin.RouterGroup, _ middleware.MiddlewareCollection, _ *handler.OrganizationHandler) {
}
