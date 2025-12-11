package router

import (
	"fmt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/builder/instrumentation"
	"opencsg.com/csghub-server/common/config"
)

func NewRouter(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	middleware.SetInfraMiddleware(r, config, instrumentation.Aigateway)
	r.Use(cors.New(cors.Config{
		AllowCredentials: true,
		AllowHeaders:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowAllOrigins:  true,
	}))
	//to access model,fintune with any kind of tokens in auth header
	r.Use(middleware.Authenticator(config))
	middlewareCollection := middleware.MiddlewareCollection{}
	middlewareCollection.Auth.NeedLogin = middleware.MustLogin()
	middlewareCollection.Auth.NeedPhoneVerified = middleware.NeedPhoneVerified(config)

	v1Group := r.Group("/v1")

	openAIhandler, err := handler.NewOpenAIHandlerFromConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating openai handler :%w", err)
	}
	v1Group.GET("/models", middlewareCollection.Auth.NeedLogin, openAIhandler.ListModels)
	v1Group.GET("/models/:model", middlewareCollection.Auth.NeedLogin, openAIhandler.GetModel)
	v1Group.POST("/chat/completions", middlewareCollection.Auth.NeedLogin, openAIhandler.Chat)
	v1Group.POST("/embeddings", middlewareCollection.Auth.NeedLogin, openAIhandler.Embedding)

	mcpProxy, err := handler.NewMCPProxyHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating mcp proxy handler :%w", err)
	}
	CreateMCPRoute(v1Group, mcpProxy)
	if err := extendRoutes(v1Group, middlewareCollection, config); err != nil {
		return nil, fmt.Errorf("error creating extended routes :%w", err)
	}
	return r, nil
}

func CreateMCPRoute(v1Group *gin.RouterGroup, mcpProxy handler.MCPProxyHandler) {
	mcpGroup := v1Group.Group("mcp")
	mcpGroup.GET("/resources", mcpProxy.Resources)

	// todo: enable mcp server proxy later
	mcpGroup.Any("/:servicename/*any", mcpProxy.ProxyToApi(""))
}
