package router

import (
	"fmt"
	"net/http"

	"opencsg.com/csghub-server/builder/instrumentation"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/i18n"
)

func NewRouter(config *config.Config) (*gin.Engine, func(), error) {
	r := gin.New()
	middleware.SetInfraMiddleware(r, config, instrumentation.Aigateway)
	r.Use(cors.New(cors.Config{
		AllowCredentials: true,
		AllowHeaders:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowAllOrigins:  true,
	}))
	store := cookie.NewStore([]byte(config.Space.SessionSecretKey))
	store.Options(sessions.Options{
		// SameSite: http.SameSiteNoneMode, // support 3rd part
		SameSite: http.SameSiteLaxMode,
		Secure:   config.EnableHTTPS,
		HttpOnly: true,
	})
	r.Use(sessions.Sessions("opencsg_jwt_session", store))
	r.Use(middleware.BuildJwtSession(config.JWT.SigningKey))
	i18n.InitLocalizersFromEmbedFile()
	r.Use(middleware.ModifyAcceptLanguageMiddleware(), middleware.LocalizedErrorMiddleware())
	r.Use(middleware.Authenticator(config))
	middlewareCollection := middleware.MiddlewareCollection{}
	middlewareCollection.Auth.NeedLogin = middleware.MustLogin()
	middlewareCollection.Auth.NeedAdmin = middleware.NeedAdmin(config)
	middlewareCollection.Auth.NeedPhoneVerified = middleware.NeedPhoneVerified(config)
	middlewareCollection.Auth.NeedAccessToken = middleware.NeedAccessToken()
	middlewareCollection.License.Check = middleware.CheckLicense(config)

	v1Group := r.Group("/v1")

	openAIhandler, err := handler.NewOpenAIHandlerFromConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating openai handler :%w", err)
	}
	v1Group.GET("/models", openAIhandler.ListModels)
	v1Group.GET("/models/*model", middlewareCollection.Auth.NeedLogin, openAIhandler.GetModel)
	v1Group.POST("/chat/completions", middlewareCollection.Auth.NeedLogin, openAIhandler.Chat)
	v1Group.POST("/embeddings", middlewareCollection.Auth.NeedLogin, openAIhandler.Embedding)
	v1Group.POST("/images/generations", middlewareCollection.Auth.NeedLogin, openAIhandler.GenerateImage)
	v1Group.POST("/audio/transcriptions", middlewareCollection.Auth.NeedLogin, openAIhandler.Transcription)

	apiV1Group := r.Group("/api/v1")
	adminGroup := apiV1Group.Group("/admin", middlewareCollection.Auth.NeedAdmin)

	mcpProxy, err := handler.NewMCPProxyHandler(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating mcp proxy handler :%w", err)
	}
	createMCPRoute(v1Group, mcpProxy)

	cleanup, err := extendRoutes(v1Group, apiV1Group, adminGroup, middlewareCollection, config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating extended routes :%w", err)
	}

	return r, cleanup, nil
}

func createMCPRoute(v1Group *gin.RouterGroup, mcpProxy handler.MCPProxyHandler) {
	mcpGroup := v1Group.Group("mcp")
	mcpGroup.GET("/resources", mcpProxy.Resources)

	mcpGroup.Any("/:servicename/*any", mcpProxy.ProxyToApi(""))
}
