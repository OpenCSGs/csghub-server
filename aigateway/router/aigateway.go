package router

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	bldprometheus "opencsg.com/csghub-server/builder/prometheus"

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

	bldprometheus.InitMetrics()
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	middlewareCollection := middleware.MiddlewareCollection{}
	middlewareCollection.Auth.NeedLogin = middleware.MustLogin()
	middlewareCollection.Auth.NeedAdmin = middleware.NeedAdmin(config)
	middlewareCollection.Auth.NeedPhoneVerified = middleware.NeedPhoneVerified(config)
	middlewareCollection.Auth.NeedAccessToken = middleware.NeedAccessToken()
	middlewareCollection.License.Check = middleware.CheckLicense(config)
	middlewareCollection.Auth.MustUserOrgApiKey = middleware.MustUserOrgApiKey(config)

	modalAPIRateLimiter := middleware.RateLimiter(config,
		middleware.WithSlidingWindowRateLimter(config),
		middleware.WithRateLimitConfig(
			config.AIGateway.ModalAPIRateLimiter.Enable,
			config.AIGateway.ModalAPIRateLimiter.Limit,
			config.AIGateway.ModalAPIRateLimiter.Window,
		),
		middleware.WithOnLimitExceeded(aigatewayRateLimitHandler),
	)

	v1Group := r.Group("/v1")

	openAIhandler, err := handler.NewOpenAIHandlerFromConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating openai handler :%w", err)
	}

	v1Group.GET("/models", openAIhandler.ListModels)
	v1Group.GET("/models/*model", openAIhandler.GetModel)
	v1Group.POST("/responses", middlewareCollection.Auth.MustUserOrgApiKey, openAIhandler.Responses)
	v1Group.POST("/chat/completions", middlewareCollection.Auth.MustUserOrgApiKey, openAIhandler.Chat)
	v1Group.POST("/embeddings", middlewareCollection.Auth.MustUserOrgApiKey, openAIhandler.Embedding)
	v1Group.POST("/rerank", middlewareCollection.Auth.MustUserOrgApiKey, openAIhandler.Rerank)
	v1Group.POST("/images/generations", middlewareCollection.Auth.MustUserOrgApiKey, modalAPIRateLimiter, openAIhandler.GenerateImage)
	v1Group.POST("/images/edits", middlewareCollection.Auth.MustUserOrgApiKey, modalAPIRateLimiter, openAIhandler.EditImage)
	v1Group.POST("/audio/transcriptions", middlewareCollection.Auth.MustUserOrgApiKey, openAIhandler.Transcription)
	v1Group.POST("/audio/speech", middlewareCollection.Auth.MustUserOrgApiKey, modalAPIRateLimiter, openAIhandler.Speech)
	v1Group.POST("/audio/speech/batch", middlewareCollection.Auth.MustUserOrgApiKey, modalAPIRateLimiter, openAIhandler.SpeechBatch)
	v1Group.GET("/audio/voices", middlewareCollection.Auth.MustUserOrgApiKey, modalAPIRateLimiter, openAIhandler.ListVoices)
	v1Group.POST("/audio/voices", middlewareCollection.Auth.MustUserOrgApiKey, modalAPIRateLimiter, openAIhandler.UploadVoice)
	v1Group.PUT("/audio/voices", middlewareCollection.Auth.MustUserOrgApiKey, modalAPIRateLimiter, openAIhandler.UpdateVoice)
	v1Group.DELETE("/audio/voices/:name", middlewareCollection.Auth.MustUserOrgApiKey, modalAPIRateLimiter, openAIhandler.DeleteVoice)
	v1Group.POST("/videos", middlewareCollection.Auth.MustUserOrgApiKey, modalAPIRateLimiter, openAIhandler.CreateVideo)
	v1Group.POST("/ocr", middlewareCollection.Auth.MustUserOrgApiKey, modalAPIRateLimiter, openAIhandler.OCR)
	v1Group.GET("/videos/:video_id", middlewareCollection.Auth.MustUserOrgApiKey, openAIhandler.GetVideo)
	v1Group.GET("/videos/:video_id/content", middlewareCollection.Auth.MustUserOrgApiKey, modalAPIRateLimiter, openAIhandler.GetVideoContent)

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
	cleanup = combineCleanup(cleanup, func() {
		_ = openAIhandler.Shutdown(context.Background())
	})

	return r, cleanup, nil
}

func combineCleanup(cleanups ...func()) func() {
	return func() {
		for _, cleanup := range cleanups {
			if cleanup != nil {
				cleanup()
			}
		}
	}
}

func createMCPRoute(v1Group *gin.RouterGroup, mcpProxy handler.MCPProxyHandler) {
	mcpGroup := v1Group.Group("mcp")
	mcpGroup.GET("/resources", mcpProxy.Resources)

	mcpGroup.Any("/:servicename/*any", mcpProxy.ProxyToApi(""))
}
