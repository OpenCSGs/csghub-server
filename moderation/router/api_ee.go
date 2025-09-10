//go:build ee || saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/moderation/handler"
)

func createAdminRoutes(adminGroup *gin.RouterGroup) {
	wordSetsHandler := handler.NewSensitiveWordSetHandler()
	wordSetsGroup := adminGroup.Group("/wordsets")
	{
		wordSetsGroup.GET("", wordSetsHandler.Index)
		wordSetsGroup.POST("", wordSetsHandler.Create)
		wordSetsGroup.GET("/:id", wordSetsHandler.Show)
		wordSetsGroup.PUT("/:id", wordSetsHandler.Update)
	}
}
