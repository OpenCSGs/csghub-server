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
	gwmw "opencsg.com/csghub-server/aigateway/middleware"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/i18n"
)

// NewRouter initializes handlers and registers authenticated service routes.
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
	r.Use(middleware.Authenticator(config))

	bldprometheus.InitMetrics()
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	middlewareCollection := middleware.MiddlewareCollection{}
	middlewareCollection.Auth.NeedLogin = middleware.MustLogin()
	middlewareCollection.Auth.NeedAdmin = middleware.NeedAdmin(config)
	middlewareCollection.Auth.PlatformAdmin = middleware.NewPlatformAdmin()
	middlewareCollection.Auth.NeedPhoneVerified = middleware.NeedPhoneVerified(config)
	middlewareCollection.Auth.NeedAccessToken = middleware.NeedAccessToken()
	middlewareCollection.License.Check = middleware.CheckLicense(config)
	middlewareCollection.Auth.MustUserOrgApiKey = middleware.MustUserOrgApiKey(config)
	APIKeyMW := middlewareCollection.Auth.MustUserOrgApiKey

	v1Group := r.Group("/v1")

	openAIhandler, err := handler.NewOpenAIHandlerFromConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating openai handler :%w", err)
	}
	routerInitialized := false
	defer func() {
		if !routerInitialized {
			_ = openAIhandler.Shutdown(context.Background())
		}
	}()

	// AnthropicHandlerImpl embeds OpenAIHandlerImpl and adds the
	// Anthropic Messages protocol on top of the shared infrastructure.
	anthropicHandler := handler.NewAnthropicHandler(openAIhandler)
	rerankHandler := handler.NewRerankHandler(openAIhandler)
	embeddingHandler := handler.NewEmbeddingHandler(openAIhandler)
	ocrHandler := handler.NewOcrHandler(openAIhandler)
	audioHandler := handler.NewAudioHandler(openAIhandler)
	speechHandler := handler.NewSpeechHandler(openAIhandler)
	imageHandler := handler.NewImageHandler(openAIhandler)
	videoHandler := handler.NewVideoHandler(openAIhandler)
	responsesHandler := handler.NewResponsesHandler(openAIhandler)
	chatHandler := handler.NewChatHandler(openAIhandler)
	jevHandler := handler.NewJevHandler(openAIhandler)
	// Metrics middleware: manages request lifecycle metrics for inference
	// routes.  Each route opts in by including metricsMw in its handler chain.
	// Returns a no-op handler + cleanup when the metrics feature is not
	// compiled in (ce build).
	metricsMW, metricsCleanup := newMetricsMiddleware(config)
	// quota limit middleware
	quotaMW := gwmw.NewApiKeyQuotaMiddleware(config)

	// api key scope resolution middleware: records the model/priority scope
	// on the media inference routes that do not run the quota middleware, so
	// the planner's model scope guard sees the same scope there. It never
	// parses request bodies and never enforces usage limits.
	scopeMW := gwmw.NewApiKeyScopeMiddleware(config)

	// text token routes
	// Also mount the Anthropic Messages API under /anthropic/v1 to match the
	// official Anthropic API path layout. Both /v1/messages and
	// /anthropic/v1/messages are supported.
	anthropicGroup := r.Group("/anthropic/v1")
	anthropicGroup.POST("/messages", APIKeyMW, quotaMW, metricsMW, anthropicHandler.Messages)

	// The scope middleware feeds the API key model scope consumed by
	// ListModels/GetModel. The full quota middleware would bring usage
	// limits (429) into these metadata endpoints and probe the user
	// service with non-API-key tokens (JWT/OAuth), so only scope is
	// resolved here; non-API-key identities pass through unrestricted.
	v1Group.GET("/models", scopeMW, openAIhandler.ListModels)
	v1Group.GET("/models/*model", scopeMW, openAIhandler.GetModel)
	v1Group.POST("/responses", APIKeyMW, quotaMW, metricsMW, responsesHandler.Responses)
	v1Group.POST("/chat/completions", APIKeyMW, quotaMW, metricsMW, chatHandler.Chat)
	v1Group.POST("/systemone", APIKeyMW, quotaMW, metricsMW, jevHandler.SystemOne)
	v1Group.POST("/messages", APIKeyMW, quotaMW, metricsMW, anthropicHandler.Messages)
	v1Group.POST("/embeddings", APIKeyMW, quotaMW, metricsMW, embeddingHandler.Embedding)
	v1Group.POST("/rerank", APIKeyMW, quotaMW, metricsMW, rerankHandler.Rerank)

	// image and audio and video routes
	v1Group.POST("/images/generations", APIKeyMW, scopeMW, metricsMW, imageHandler.GenerateImage)
	v1Group.POST("/images/edits", APIKeyMW, scopeMW, metricsMW, imageHandler.EditImage)
	v1Group.POST("/audio/transcriptions", APIKeyMW, scopeMW, metricsMW, audioHandler.Transcription)
	v1Group.POST("/audio/speech", APIKeyMW, scopeMW, metricsMW, speechHandler.Speech)
	v1Group.POST("/audio/speech/batch", APIKeyMW, scopeMW, metricsMW, speechHandler.SpeechBatch)
	v1Group.GET("/audio/voices", APIKeyMW, openAIhandler.ListVoices)
	v1Group.POST("/audio/voices", APIKeyMW, openAIhandler.UploadVoice)
	v1Group.PUT("/audio/voices", APIKeyMW, openAIhandler.UpdateVoice)
	v1Group.DELETE("/audio/voices/:name", APIKeyMW, openAIhandler.DeleteVoice)
	v1Group.POST("/videos", APIKeyMW, scopeMW, metricsMW, videoHandler.CreateVideo)
	v1Group.GET("/videos/:video_id", APIKeyMW, metricsMW, videoHandler.GetVideo)
	v1Group.GET("/videos/:video_id/content", APIKeyMW, metricsMW, videoHandler.GetVideoContent)
	v1Group.POST("/video/generations", APIKeyMW, scopeMW, metricsMW, videoHandler.CreateVideo)
	v1Group.GET("/video/generations/:video_id", APIKeyMW, metricsMW, videoHandler.GetVideo)
	v1Group.GET("/video/generations/:video_id/content", APIKeyMW, metricsMW, videoHandler.GetVideoContent)
	v1Group.POST("/ocr", APIKeyMW, scopeMW, metricsMW, ocrHandler.OCR)

	apiV1Group := r.Group("/api/v1")
	adminGroup := apiV1Group.Group("/admin")
	// Only admin routes need localized error messages; customer-facing
	// inference APIs skip i18n initialization entirely.
	i18n.InitLocalizersFromEmbedFile()
	adminGroup.Use(middleware.ModifyAcceptLanguageMiddleware(), middleware.LocalizedErrorMiddleware())
	createAdminTokenPlanModelRoutes(adminGroup, middlewareCollection, openAIhandler, scopeMW)

	mcpProxy, err := handler.NewMCPProxyHandler(config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating mcp proxy handler :%w", err)
	}
	createMCPRoute(v1Group, mcpProxy)

	cleanup, err := extendRoutes(v1Group, apiV1Group, adminGroup, middlewareCollection, config)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating extended routes :%w", err)
	}
	cleanup = combineCleanup(cleanup, metricsCleanup)
	cleanup = combineCleanup(cleanup, func() {
		_ = openAIhandler.Shutdown(context.Background())
	})

	routerInitialized = true
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
