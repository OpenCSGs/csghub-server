package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

func NewRouter(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log(config))
	//to access model,fintune with any kind of tokens in auth header
	r.Use(middleware.Authenticator(config))
	mustLogin := middleware.MustLogin()

	v1Group := r.Group("/v1")

	openAIhandler, err := handler.NewOpenAIHandlerFromConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating openai handler :%w", err)
	}
	v1Group.GET("/models", mustLogin, openAIhandler.ListModels)
	v1Group.GET("/models/:model", mustLogin, openAIhandler.GetModel)
	v1Group.POST("/chat/completions", mustLogin, openAIhandler.Chat)
	v1Group.POST("/embeddings", mustLogin, openAIhandler.Embedding)

	mcpProxy, err := handler.NewMCPProxyHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating mcp proxy handler :%w", err)
	}
	CreateMCPRoute(v1Group, mcpProxy)
	return r, nil
}

func CreateMCPRoute(v1Group *gin.RouterGroup, mcpProxy handler.MCPProxyHandler) {
	mcpGroup := v1Group.Group("mcp")
	mcpGroup.GET("/servers", mcpProxy.List)

	// todo: enable mcp server proxy later
	mcpGroup.Any("/:servicename/*any", mcpProxy.ProxyToApi(""))
}
