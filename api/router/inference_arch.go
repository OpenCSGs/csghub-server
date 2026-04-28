package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
)

// createInferenceArchRoutes creates routes for inference arch
func createInferenceArchRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection) {
	inferenceArchHandler, _ := handler.NewInferenceArchHandler()

	apiGroup.GET("/inference_arch", middlewareCollection.Auth.NeedAdmin, inferenceArchHandler.GetInferenceArch)
	apiGroup.PUT("/inference_arch", middlewareCollection.Auth.NeedAdmin, inferenceArchHandler.UpdateInferenceArch)
}
