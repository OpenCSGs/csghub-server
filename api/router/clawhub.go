package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

// createClawHubRoutes registers ClawHub Registry routes
func createClawHubRoutes(r *gin.Engine, apiGroup *gin.RouterGroup, config *config.Config) error {
	clawhubHandler, err := handler.NewClawHubHandler(config)
	if err != nil {
		return err
	}

	// ClawHub v1 routes - use /claw/api/v1 prefix
	clawhubGroup := r.Group("/claw/api/v1")

	// Core APIs: search, publish, install, login
	clawhubGroup.GET("/search", clawhubHandler.Search)
	clawhubGroup.POST("/skills", middleware.MustLogin(), clawhubHandler.PublishSkill)
	clawhubGroup.GET("/skills/:slug/versions/:version", clawhubHandler.GetSkillVersion)
	clawhubGroup.GET("/skills/:slug", clawhubHandler.GetSkill)
	clawhubGroup.GET("/download/:slug", clawhubHandler.DownloadSkillByPath)
	clawhubGroup.GET("/download", clawhubHandler.DownloadSkill)
	clawhubGroup.GET("/resolve/:slug", clawhubHandler.ResolveSkillByPath)
	clawhubGroup.GET("/resolve", clawhubHandler.ResolveSkill)
	clawhubGroup.GET("/whoami", clawhubHandler.Whoami)

	return nil
}
