package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
)

// createAgentRoutes sets up the routes for agent-related API endpoints
func createAgentRoutes(
	apiGroup *gin.RouterGroup,
	middlewareCollection middleware.MiddlewareCollection,
	agentHandler *handler.AgentHandler,
) {
	agentGroup := apiGroup.Group("/agent")
	agentGroup.Use(middlewareCollection.Auth.NeedLogin)

	// Template routes
	templatesGroup := agentGroup.Group("/templates")
	{
		templatesGroup.GET("", agentHandler.ListTemplates)
		templatesGroup.POST("", agentHandler.CreateTemplate)
		templatesGroup.GET("/:id", agentHandler.GetTemplate)
		templatesGroup.PUT("/:id", agentHandler.UpdateTemplate)
		templatesGroup.DELETE("/:id", agentHandler.DeleteTemplate)

		// Instance routes by template
		templatesGroup.GET("/:id/instances", agentHandler.ListInstancesByTemplate)
	}

	// Instance routes
	instancesGroup := agentGroup.Group("/instances")
	{
		instancesGroup.GET("", agentHandler.ListInstances)
		instancesGroup.POST("", agentHandler.CreateInstance)
		instancesGroup.GET("/:id", agentHandler.GetInstance)
		instancesGroup.PUT("/:id", agentHandler.UpdateInstance)
		instancesGroup.PUT("/by-content-id/:type/*content_id", agentHandler.UpdateInstanceByContentID)
		instancesGroup.DELETE("/:id", agentHandler.DeleteInstance)
		instancesGroup.DELETE("/by-content-id/:type/*content_id", agentHandler.DeleteInstanceByContentID)

		// Session routes (nested under instances)
		sessionsGroup := instancesGroup.Group("/:id/sessions")
		{
			sessionsGroup.GET("", agentHandler.ListSessions)
			sessionsGroup.POST("", agentHandler.CreateSession)
			sessionsGroup.GET("/:session_uuid", agentHandler.GetSession)
			sessionsGroup.DELETE("/:session_uuid", agentHandler.DeleteSession)
			sessionsGroup.PUT("/:session_uuid", agentHandler.UpdateSession)

			// Session histories routes
			historiesGroup := sessionsGroup.Group("/:session_uuid/histories")
			{
				historiesGroup.POST("", agentHandler.CreateSessionHistory)
				historiesGroup.GET("", agentHandler.ListSessionHistories)
				historiesGroup.PUT("/:msg_uuid/feedback", agentHandler.UpdateSessionHistoryFeedback)
				historiesGroup.PUT("/:msg_uuid/rewrite", agentHandler.RewriteMessage)
			}
		}
	}

}
