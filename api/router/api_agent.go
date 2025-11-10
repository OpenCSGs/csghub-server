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

	// Template routes
	templatesGroup := agentGroup.Group("/templates")
	{
		templatesGroup.GET("", middlewareCollection.Auth.NeedLogin, agentHandler.ListTemplates)
		templatesGroup.POST("", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.CreateTemplate)
		templatesGroup.GET("/:id", middlewareCollection.Auth.NeedLogin, agentHandler.GetTemplate)
		templatesGroup.PUT("/:id", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.UpdateTemplate)
		templatesGroup.DELETE("/:id", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.DeleteTemplate)

		// Instance routes by template
		templatesGroup.GET("/:id/instances", middlewareCollection.Auth.NeedLogin, agentHandler.ListInstancesByTemplate)
	}

	// Instance routes
	instancesGroup := agentGroup.Group("/instances")
	{
		instancesGroup.GET("", middlewareCollection.Auth.NeedLogin, agentHandler.ListInstances)
		instancesGroup.POST("", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.CreateInstance)
		instancesGroup.GET("/:id", middlewareCollection.Auth.NeedLogin, agentHandler.GetInstance)
		instancesGroup.PUT("/:id", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.UpdateInstance)
		instancesGroup.PUT("/by-content-id/:type/*content_id", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.UpdateInstanceByContentID)
		instancesGroup.DELETE("/:id", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.DeleteInstance)
		instancesGroup.DELETE("/by-content-id/:type/*content_id", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.DeleteInstanceByContentID)

		// Session routes (nested under instances)
		sessionsGroup := instancesGroup.Group("/:id/sessions")
		{
			sessionsGroup.GET("", middlewareCollection.Auth.NeedLogin, agentHandler.ListSessions)
			sessionsGroup.POST("", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.CreateSession)
			sessionsGroup.GET("/:session_uuid", middlewareCollection.Auth.NeedLogin, agentHandler.GetSession)
			sessionsGroup.DELETE("/:session_uuid", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.DeleteSession)
			sessionsGroup.PUT("/:session_uuid", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.UpdateSession)

			// Session histories routes
			historiesGroup := sessionsGroup.Group("/:session_uuid/histories")
			{
				historiesGroup.POST("", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.CreateSessionHistory)
				historiesGroup.GET("", middlewareCollection.Auth.NeedLogin, agentHandler.ListSessionHistories)
				historiesGroup.PUT("/:msg_uuid/feedback", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.UpdateSessionHistoryFeedback)
				historiesGroup.PUT("/:msg_uuid/rewrite", middlewareCollection.Auth.NeedPhoneVerified, agentHandler.RewriteMessage)
			}
		}
	}

}
