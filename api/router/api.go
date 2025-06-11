package router

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	cache "github.com/chenyahui/gin-cache"
	"github.com/chenyahui/gin-cache/persist"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/handler/callback"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/builder/instrumentation"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/i18n"
	"opencsg.com/csghub-server/common/types"
)

func RunServer(config *config.Config, enableSwagger bool) {
	stopOtel, err := instrumentation.SetupOTelSDK(context.Background(), config, "csghub-api")
	if err != nil {
		panic(err)
	}
	r, err := NewRouter(config, enableSwagger)
	if err != nil {
		panic(err)
	}
	slog.Info("csghub service is running", slog.Any("port", config.APIServer.Port))
	server := httpbase.NewGracefulServer(
		httpbase.GraceServerOpt{
			Port: config.APIServer.Port,
		},
		r,
	)

	i18n.InitLocalizersFromEmbedFile()
	server.Run()
	_ = stopOtel(context.Background())
	temporal.Stop()

}

func NewRouter(config *config.Config, enableSwagger bool) (*gin.Engine, error) {
	r := gin.New()
	if config.Instrumentation.OTLPEndpoint != "" {
		r.Use(otelgin.Middleware("csghub-server"))
	}

	r.Use(cors.New(cors.Config{
		AllowCredentials: true,
		AllowHeaders:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowAllOrigins:  true,
	}))
	r.Use(gin.Recovery())
	r.Use(middleware.Log(config))
	r.Use(middleware.ModifyAcceptLanguageMiddleware())
	r.Use(middleware.LocalizedErrorMiddleware())
	gitHTTPHandler, err := handler.NewGitHTTPHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating git http handler:%w", err)
	}
	r.POST("/api/v1/complete_multipart", gitHTTPHandler.CompleteMultipartUpload)
	gitHTTP := r.Group("/:repo_type/:namespace/:name")
	gitHTTP.Use(middleware.GitHTTPParamMiddleware())
	gitHTTP.Use(middleware.GetCurrentUserFromHeader())
	{
		gitHTTP.GET("/info/refs", gitHTTPHandler.InfoRefs)
		gitHTTP.POST("/git-upload-pack", middleware.ContentEncoding(), gitHTTPHandler.GitUploadPack)
		gitHTTP.POST("/git-receive-pack", middleware.ContentEncoding(), gitHTTPHandler.GitReceivePack)
		lfsGroup := gitHTTP.Group("/info/lfs")
		{
			objectsGroup := lfsGroup.Group("/objects")
			{
				objectsGroup.POST("/batch", gitHTTPHandler.LfsBatch)
				objectsGroup.PUT("/:oid/:size", gitHTTPHandler.LfsUpload)
				lfsGroup.GET("/:oid", gitHTTPHandler.LfsDownload)
			}
			lfsGroup.POST("/verify", gitHTTPHandler.LfsVerify)

			locksGroup := lfsGroup.Group("/locks")
			{
				locksGroup.GET("", gitHTTPHandler.ListLocks)
				locksGroup.POST("", gitHTTPHandler.CreateLock)
				locksGroup.POST("/verify", gitHTTPHandler.VerifyLock)
				locksGroup.POST("/:lid/unlock", gitHTTPHandler.UnLock)
			}

		}

	}

	r.Use(middleware.Authenticator(config))

	middlewareCollection := middleware.MiddlewareCollection{}
	middlewareCollection.Auth.NeedAPIKey = middleware.OnlyAPIKeyAuthenticator(config)
	middlewareCollection.Auth.NeedLogin = middleware.MustLogin()
	middlewareCollection.Auth.NeedAdmin = middleware.NeedAdmin(config)
	middlewareCollection.Auth.UserMatch = middleware.UserMatch()
	middlewareCollection.Repo.RepoExists = middleware.RepoExists(config)

	if enableSwagger {
		r.GET("/api/v1/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// User routes
	userHandler, err := handler.NewUserHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	orgHandler, err := handler.NewOrganizationHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating organization controller:%w", err)
	}

	repoCommonHandler, err := handler.NewRepoHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating repo common handler: %w", err)
	}
	modelHandler, err := handler.NewModelHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating model controller:%w", err)
	}
	dsHandler, err := handler.NewDatasetHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dataset handler:%w", err)
	}

	// Mirror
	mirrorHandler, err := handler.NewMirrorHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating mirror controller:%w", err)
	}

	hfdsHandler, err := handler.NewHFDatasetHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating HF dataset handler: %w", err)
	}
	//create routes for hf
	createMappingRoutes(r, "/hf", middlewareCollection, hfdsHandler, repoCommonHandler, modelHandler, userHandler, gitHTTPHandler)
	//create routes for ms
	createMappingRoutes(r, "/ms", middlewareCollection, hfdsHandler, repoCommonHandler, modelHandler, userHandler, gitHTTPHandler)
	//create routes for csg
	createMappingRoutes(r, "/csg", middlewareCollection, hfdsHandler, repoCommonHandler, modelHandler, userHandler, gitHTTPHandler)

	apiGroup := r.Group("/api/v1")

	versionHandler := handler.NewVersionHandler()
	apiGroup.GET("/version", versionHandler.Version)

	// TODO:use middleware to handle common response
	//
	memoryStore := persist.NewMemoryStore(1 * time.Minute)

	// List trending models and datasets routes
	listHandler, err := handler.NewListHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creatring list handler: %v", err)
	}
	{
		apiGroup.POST("/list/models_by_path", cache.CacheByRequestURI(memoryStore, 1*time.Minute), listHandler.ListModelsByPath)
		apiGroup.POST("/list/datasets_by_path", cache.CacheByRequestURI(memoryStore, 1*time.Minute), listHandler.ListDatasetsByPath)
		apiGroup.POST("/list/spaces_by_path", cache.CacheByRequestURI(memoryStore, 1*time.Minute), listHandler.ListSpacesByPath)
	}

	//evaluation handler
	evaluationHandler, err := handler.NewEvaluationHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creatring evaluation handler: %v", err)
	}

	createEvaluationRoutes(apiGroup, middlewareCollection, evaluationHandler)

	// monitor handler
	monitorHandler, err := handler.NewMonitorHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creatring monitor handler: %v", err)
	}

	// Model routes
	createModelRoutes(config, apiGroup, middlewareCollection, modelHandler, repoCommonHandler, monitorHandler)

	// Dataset routes
	createDatasetRoutes(config, apiGroup, middlewareCollection, dsHandler, repoCommonHandler)

	codeHandler, err := handler.NewCodeHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating code handler:%w", err)
	}
	// Code routes
	createCodeRoutes(config, apiGroup, middlewareCollection, codeHandler, repoCommonHandler)

	spaceHandler, err := handler.NewSpaceHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating space handler:%w", err)
	}
	// space routers
	createSpaceRoutes(config, apiGroup, middlewareCollection, spaceHandler, repoCommonHandler, monitorHandler)

	spaceResourceHandler, err := handler.NewSpaceResourceHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating space resource handler:%w", err)
	}

	spaceResource := apiGroup.Group("space_resources")
	{
		spaceResource.GET("", spaceResourceHandler.Index)
		spaceResource.POST("", middlewareCollection.Auth.NeedAdmin, spaceResourceHandler.Create)
		spaceResource.PUT("/:id", middlewareCollection.Auth.NeedAdmin, spaceResourceHandler.Update)
		spaceResource.DELETE("/:id", middlewareCollection.Auth.NeedAdmin, spaceResourceHandler.Delete)
	}

	spaceSdkHandler, err := handler.NewSpaceSdkHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating space sdk handler:%w", err)
	}

	spaceSdk := apiGroup.Group("space_sdks")
	{
		spaceSdk.GET("", spaceSdkHandler.Index)
		spaceSdk.POST("", middlewareCollection.Auth.NeedAPIKey, spaceSdkHandler.Create)
		spaceSdk.PUT("/:id", middlewareCollection.Auth.NeedAPIKey, spaceSdkHandler.Update)
		spaceSdk.DELETE("/:id", middlewareCollection.Auth.NeedAPIKey, spaceSdkHandler.Delete)
	}

	userProxyHandler, err := handler.NewInternalServiceProxyHandler(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port))
	if err != nil {
		return nil, fmt.Errorf("error creating user proxy handler:%w", err)
	}

	createUserRoutes(apiGroup, middlewareCollection, userProxyHandler, userHandler)

	tokenGroup := apiGroup.Group("token")
	{
		tokenGroup.POST("/:app/:token_name", userProxyHandler.ProxyToApi("/api/v1/token/%s/%s", "app", "token_name"))
		tokenGroup.PUT("/:app/:token_name", userProxyHandler.ProxyToApi("/api/v1/token/%s/%s", "app", "token_name"))
		tokenGroup.DELETE("/:app/:token_name", userProxyHandler.ProxyToApi("/api/v1/token/%s/%s", "app", "token_name"))
		// check token info
		tokenGroup.GET("/:token_value", middlewareCollection.Auth.NeedAPIKey, userProxyHandler.ProxyToApi("/api/v1/token/%s", "token_value"))
	}

	sshKeyHandler, err := handler.NewSSHKeyHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	{
		apiGroup.GET("/user/:username/ssh_keys", middlewareCollection.Auth.UserMatch, sshKeyHandler.Index)
		apiGroup.POST("/user/:username/ssh_keys", middlewareCollection.Auth.UserMatch, sshKeyHandler.Create)
		apiGroup.DELETE("/user/:username/ssh_key/:name", middlewareCollection.Auth.UserMatch, sshKeyHandler.Delete)
	}

	// Organization routes
	createOrgRoutes(apiGroup, userProxyHandler, orgHandler)

	// Tag
	tagCtrl, err := handler.NewTagHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating tag controller:%w", err)
	}
	createTagsRoutes(apiGroup, middlewareCollection, tagCtrl)

	// JWT token
	apiGroup.POST("/jwt/token", middlewareCollection.Auth.NeedAPIKey, userProxyHandler.Proxy)
	apiGroup.GET("/jwt/:token", middlewareCollection.Auth.NeedAPIKey, userProxyHandler.ProxyToApi("/api/v1/jwt/%s", "token"))
	apiGroup.GET("/users", userProxyHandler.Proxy)

	// callback
	callbackCtrl, err := callback.NewGitCallbackHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating callback controller:%w", err)
	}
	apiGroup.POST("/callback/git", callbackCtrl.Handle)
	apiGroup.GET("/callback/casdoor", userProxyHandler.Proxy)
	// Sensive check
	sensitiveCtrl, err := handler.NewSensitiveHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive handler:%w", err)
	}
	apiGroup.POST("/sensitive/text", sensitiveCtrl.Text)
	apiGroup.POST("/sensitive/image", sensitiveCtrl.Image)

	// MirrorSource
	msHandler, err := handler.NewMirrorSourceHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating mirror source controller:%w", err)
	}

	apiGroup.GET("/mirrors", middlewareCollection.Auth.NeedAdmin, mirrorHandler.Index)

	lfsSyncProxyHandler, err := handler.NewInternalServiceProxyHandler(fmt.Sprintf("%s:%d", config.LfsSync.Host, config.LfsSync.Port))
	if err != nil {
		return nil, fmt.Errorf("error creating multi sync proxy handler:%w", err)
	}
	createLfsSyncRoutes(apiGroup, middlewareCollection, lfsSyncProxyHandler)

	mirror := apiGroup.Group("/mirror")
	mirror.Use(middlewareCollection.Auth.NeedAdmin)
	{
		mirror.GET("/sources", msHandler.Index)
		mirror.POST("/sources", msHandler.Create)
		mirror.PUT("/sources/:id", msHandler.Update)
		mirror.DELETE("/sources/:id", msHandler.Delete)
		mirror.GET("/sources/:id", msHandler.Get)
		mirror.POST("/repo", mirrorHandler.CreateMirrorRepo)
		mirror.GET("/repos", mirrorHandler.Repos)
	}

	collectionHandler, err := handler.NewCollectionHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating collection handler:%w", err)
	}
	collections := apiGroup.Group("/collections")
	{
		// list all collection
		collections.GET("", collectionHandler.Index)
		collections.POST("", middlewareCollection.Auth.NeedLogin, collectionHandler.Create)
		collections.GET("/:id", collectionHandler.GetCollection)
		collections.PUT("/:id", middlewareCollection.Auth.NeedLogin, collectionHandler.UpdateCollection)
		collections.DELETE("/:id", middlewareCollection.Auth.NeedLogin, collectionHandler.DeleteCollection)
		collections.POST("/:id/repos", middlewareCollection.Auth.NeedLogin, collectionHandler.AddRepoToCollection)
		collections.DELETE("/:id/repos", middlewareCollection.Auth.NeedLogin, collectionHandler.RemoveRepoFromCollection)
	}

	// cluster infos
	clusterHandler, err := handler.NewClusterHandler(config)
	if err != nil {
		return nil, fmt.Errorf("fail to creating cluster handler: %w", err)
	}
	cluster := apiGroup.Group("/cluster")
	{
		cluster.GET("", middlewareCollection.Auth.NeedLogin, clusterHandler.Index)
		cluster.GET("/:id", middlewareCollection.Auth.NeedLogin, clusterHandler.GetClusterById)
		cluster.PUT("/:id", middlewareCollection.Auth.NeedAPIKey, clusterHandler.Update)
	}

	eventHandler, err := handler.NewEventHandler()
	if err != nil {
		return nil, fmt.Errorf("error creating event handler:%w", err)
	}
	event := apiGroup.Group("/events")
	event.POST("", eventHandler.Create)

	// routes for broadcast
	broadcastHandler, err := handler.NewBroadcastHandler()
	if err != nil {
		return nil, fmt.Errorf("error creating broadcast handler:%w", err)
	}
	broadcast := apiGroup.Group("/broadcasts")
	adminBroadcast := apiGroup.Group("/admin/broadcasts")
	adminBroadcast.Use(middlewareCollection.Auth.NeedAdmin)

	adminBroadcast.POST("", broadcastHandler.Create)
	adminBroadcast.PUT("/:id", broadcastHandler.Update)
	adminBroadcast.GET("", broadcastHandler.Index)
	adminBroadcast.GET("/:id", broadcastHandler.Show)
	broadcast.GET("/:id", broadcastHandler.Show)
	broadcast.GET("/active", broadcastHandler.Active)
	// end routes for broadcast

	runtimeArchHandler, err := handler.NewRuntimeArchitectureHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating runtime framework architecture handler:%w", err)
	}

	createRuntimeFrameworkRoutes(apiGroup, middlewareCollection, modelHandler, runtimeArchHandler, repoCommonHandler)

	syncHandler, err := handler.NewSyncHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sync handler:%w", err)
	}
	syncClientSettingHandler, err := handler.NewSyncClientSettingHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sync client setting handler:%w", err)
	}
	syncGroup := apiGroup.Group("sync")
	{
		syncGroup.GET("/version/latest", middlewareCollection.Auth.NeedLogin, syncHandler.Latest)
		// syncGroup.GET("/version/oldest", syncHandler.Oldest)
		syncGroup.GET("/client_setting", middlewareCollection.Auth.NeedAdmin, syncClientSettingHandler.Show)
		syncGroup.POST("/client_setting", middlewareCollection.Auth.NeedAdmin, syncClientSettingHandler.Create)
	}

	accountingHandler, err := handler.NewAccountingHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating accounting handler setting handler:%w", err)
	}

	createAccountRoutes(apiGroup, middlewareCollection, accountingHandler)

	recomHandler, err := handler.NewRecomHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating recomHandler,%w", err)
	}
	recomGroup := apiGroup.Group("/recom")
	{
		recomGroup.POST("opweight", middlewareCollection.Auth.NeedAdmin, recomHandler.SetOpWeight)
	}

	// telemetry
	telemetryHandler, err := handler.NewTelemetryHandler()
	if err != nil {
		return nil, fmt.Errorf("error creating telemetry handler:%w", err)
	}
	teleGroup := apiGroup.Group("/telemetry")
	teleGroup.POST("/usage", telemetryHandler.Usage)

	// internal API for gitaly to check request permissions
	internalHandler, err := handler.NewInternalHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating internalHandler,%w", err)
	}
	needGitlabShellJWTToken := middleware.CheckGitlabShellJWTToken(config)
	r.GET("/api/v4/internal/authorized_keys", needGitlabShellJWTToken, internalHandler.GetAuthorizedKeys)
	r.POST("/api/v4/internal/allowed", needGitlabShellJWTToken, internalHandler.SSHAllowed)
	r.POST("/api/v4/internal/pre_receive", needGitlabShellJWTToken, internalHandler.PreReceive)
	r.POST("api/v4/internal/lfs_authenticate", needGitlabShellJWTToken, internalHandler.LfsAuthenticate)
	r.POST("/api/v4/internal/post_receive", needGitlabShellJWTToken, internalHandler.PostReceive)
	internalGroup := apiGroup.Group("/internal")
	{
		internalGroup.POST("/allowed", needGitlabShellJWTToken, internalHandler.Allowed)
		internalGroup.POST("/pre_receive", needGitlabShellJWTToken, internalHandler.PreReceive)
		internalGroup.POST("/post_receive", needGitlabShellJWTToken, internalHandler.PostReceive)
	}

	discussionHandler, err := handler.NewDiscussionHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating discussion handler:%w", err)
	}
	createDiscussionRoutes(apiGroup, middlewareCollection, discussionHandler)

	// prompt
	promptHandler, err := handler.NewPromptHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating prompt handler,%w", err)
	}
	createPromptRoutes(apiGroup, middlewareCollection, promptHandler, repoCommonHandler)

	// dataflow proxy
	dataflowHandler, err := handler.NewDataflowProxyHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating data flow proxy handler:%w", err)
	}
	createDataflowRoutes(apiGroup, dataflowHandler)

	// Dataset viewer proxy
	dataViewerAddr := fmt.Sprintf("%s:%d", config.DataViewer.Host, config.DataViewer.Port)
	dsViewerHandler, err := handler.NewInternalServiceProxyHandler(dataViewerAddr)
	if err != nil {
		return nil, fmt.Errorf("error creating dataset viewer proxy:%w", err)
	}

	createDataViewerRoutes(apiGroup, middlewareCollection, dsViewerHandler)

	// space template
	templateHandler, err := handler.NewSpaceTemplateHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating space template proxy:%w", err)
	}
	createSpaceTemplateRoutes(apiGroup, middlewareCollection, templateHandler)

	// mcp server
	mcpHandler, err := handler.NewMCPServerHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating mcp server handler: %w", err)
	}
	CreateMCPServerRoutes(apiGroup, middlewareCollection, mcpHandler, repoCommonHandler)

	if err := createNotificationRoutes(config, apiGroup, middlewareCollection); err != nil {
		return nil, fmt.Errorf("error creating notifier routes: %w", err)
	}
	return r, nil
}

func createEvaluationRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, evaluationHandler *handler.EvaluationHandler) {
	// Models routes
	evaluationsGroup := apiGroup.Group("/evaluations")
	evaluationsGroup.Use(middlewareCollection.Auth.NeedLogin)
	{
		evaluationsGroup.POST("", evaluationHandler.RunEvaluation)
		evaluationsGroup.DELETE("/:id", evaluationHandler.DeleteEvaluation)
		evaluationsGroup.GET("/:id", evaluationHandler.GetEvaluation)
	}
}

func createModelRoutes(config *config.Config,
	apiGroup *gin.RouterGroup,
	middlewareCollection middleware.MiddlewareCollection,
	modelHandler *handler.ModelHandler,
	repoCommonHandler *handler.RepoHandler,
	monitorHandler *handler.MonitorHandler) {
	// Models routes
	modelsGroup := apiGroup.Group("/models")
	modelsGroup.Use(middleware.RepoType(types.ModelRepo), middlewareCollection.Repo.RepoExists)
	{
		modelsGroup.POST("", middlewareCollection.Auth.NeedLogin, modelHandler.Create)
		modelsGroup.GET("", modelHandler.Index)
		modelsGroup.PUT("/:namespace/:name", middlewareCollection.Auth.NeedLogin, modelHandler.Update)
		modelsGroup.DELETE("/:namespace/:name", middlewareCollection.Auth.NeedLogin, modelHandler.Delete)
		modelsGroup.GET("/:namespace/:name", modelHandler.Show)
		modelsGroup.GET("/:namespace/:name/all_files", repoCommonHandler.AllFiles)
		modelsGroup.GET("/:namespace/:name/relations", modelHandler.Relations)
		modelsGroup.PUT("/:namespace/:name/relations", middlewareCollection.Auth.NeedAdmin, modelHandler.SetRelations)
		modelsGroup.POST("/:namespace/:name/relations/dataset", middlewareCollection.Auth.NeedAdmin, modelHandler.AddDatasetRelation)
		modelsGroup.DELETE("/:namespace/:name/relations/dataset", middlewareCollection.Auth.NeedAdmin, modelHandler.DelDatasetRelation)
		modelsGroup.GET("/:namespace/:name/quantizations", modelHandler.ListQuantizations)
	}

	// Models repo operation routes
	{
		modelsGroup.GET("/:namespace/:name/branches", repoCommonHandler.Branches)
		modelsGroup.GET("/:namespace/:name/tags", repoCommonHandler.Tags)
		modelsGroup.POST("/:namespace/:name/preupload/:revision", repoCommonHandler.Preupload)
		// update tags of a certain category
		modelsGroup.POST("/:namespace/:name/tags/:category", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateTags)
		modelsGroup.GET("/:namespace/:name/last_commit", repoCommonHandler.LastCommit)
		modelsGroup.GET("/:namespace/:name/commit/:commit_id", repoCommonHandler.CommitWithDiff)
		modelsGroup.POST("/:namespace/:name/commit/:revision", repoCommonHandler.CommitFiles)
		modelsGroup.GET("/:namespace/:name/remote_diff", repoCommonHandler.RemoteDiff)
		modelsGroup.GET("/:namespace/:name/tree", repoCommonHandler.Tree)
		modelsGroup.GET("/:namespace/:name/refs/:ref/tree/*path", repoCommonHandler.TreeV2)
		modelsGroup.GET("/:namespace/:name/refs/:ref/logs_tree/*path", repoCommonHandler.LogsTree)
		modelsGroup.GET("/:namespace/:name/commits", repoCommonHandler.Commits)
		modelsGroup.GET("/:namespace/:name/raw/*file_path", repoCommonHandler.FileRaw)
		modelsGroup.DELETE("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeleteFile)
		modelsGroup.GET("/:namespace/:name/blob/*file_path", repoCommonHandler.FileInfo)
		// The DownloadFile method differs from the SDKDownload interface in a few ways

		// 1.When passing the file_path parameter to the SDKDownload method,
		// it only needs to pass the path of the file itself,
		// whether it is an lfs file or a non-lfs file.
		// The DownloadFile has a different file_path format for lfs files and non-lfs files,
		// and an lfs parameter needs to be added.
		// 2. DownloadFile returns an object store url for lfs files, while SDKDownload redirects directly.
		modelsGroup.GET("/:namespace/:name/download/*file_path", repoCommonHandler.DownloadFile)
		modelsGroup.GET("/:namespace/:name/resolve/*file_path", repoCommonHandler.ResolveDownload)
		modelsGroup.POST("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.CreateFile)
		modelsGroup.PUT("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateFile)
		modelsGroup.POST("/:namespace/:name/update_downloads", repoCommonHandler.UpdateDownloads)
		modelsGroup.PUT("/:namespace/:name/incr_downloads", repoCommonHandler.IncrDownloads)
		modelsGroup.POST("/:namespace/:name/upload_file", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UploadFile)
		modelsGroup.POST("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.CreateMirror)
		modelsGroup.GET("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.GetMirror)
		modelsGroup.PUT("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateMirror)
		modelsGroup.DELETE("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeleteMirror)
		modelsGroup.POST("/:namespace/:name/mirror/sync", middlewareCollection.Auth.NeedLogin, repoCommonHandler.SyncMirror)

		// mirror from SaaS, only on-premises available
		if !config.Saas {
			modelsGroup.POST("/:namespace/:name/mirror_from_saas", middlewareCollection.Auth.NeedLogin, repoCommonHandler.MirrorFromSaas)
		}

		// runtime framework
		modelsGroup.GET("/:namespace/:name/runtime_framework", repoCommonHandler.RuntimeFrameworkList)
		// runtime framework for both finetune and inference
		modelsGroup.GET("/runtime_framework", middlewareCollection.Auth.NeedLogin, repoCommonHandler.RuntimeFrameworkListWithType)
	}

	modelsDeployGroup := modelsGroup.Group("")
	modelsDeployGroup.Use(middlewareCollection.Auth.NeedLogin)
	{
		// list model inference
		modelsDeployGroup.GET("/:namespace/:name/run", repoCommonHandler.DeployList)
		// deploy model as inference
		modelsDeployGroup.POST("/:namespace/:name/run", modelHandler.DeployDedicated)
		// delete a deployed inference
		modelsDeployGroup.DELETE("/:namespace/:name/run/:id", modelHandler.DeployDelete)
		modelsDeployGroup.GET("/:namespace/:name/run/:id", repoCommonHandler.DeployDetail)
		modelsDeployGroup.GET("/:namespace/:name/run/:id/status", repoCommonHandler.DeployStatus)
		modelsDeployGroup.GET("/:namespace/:name/run/:id/logs/:instance", repoCommonHandler.DeployInstanceLogs)
		modelsDeployGroup.PUT("/:namespace/:name/run/:id", repoCommonHandler.DeployUpdate)
		modelsDeployGroup.PUT("/:namespace/:name/run/:id/stop", modelHandler.DeployStop)
		modelsDeployGroup.PUT("/:namespace/:name/run/:id/start", modelHandler.DeployStart)

		// deploy model as finetune instance
		modelsDeployGroup.POST("/:namespace/:name/finetune", modelHandler.FinetuneCreate)
		// stop a finetune instance
		modelsDeployGroup.PUT("/:namespace/:name/finetune/:id/stop", modelHandler.FinetuneStop)
		// start a finetune instance
		modelsDeployGroup.PUT("/:namespace/:name/finetune/:id/start", modelHandler.FinetuneStart)
		// delete a finetune instance
		modelsDeployGroup.DELETE("/:namespace/:name/finetune/:id", modelHandler.FinetuneDelete)
	}

	modelsMonitorGroup := modelsGroup.Group("")
	modelsMonitorGroup.Use(middlewareCollection.Auth.NeedLogin)
	{
		// inference monitor
		modelsMonitorGroup.GET("/:namespace/:name/run/:id/cpu/:instance/usage", monitorHandler.CPUUsage)
		modelsMonitorGroup.GET("/:namespace/:name/run/:id/memory/:instance/usage", monitorHandler.MemoryUsage)
		modelsMonitorGroup.GET("/:namespace/:name/run/:id/request/:instance/count", monitorHandler.RequestCount)
		modelsMonitorGroup.GET("/:namespace/:name/run/:id/request/:instance/latency", monitorHandler.RequestLatency)

		// finetune monitor
		modelsMonitorGroup.GET("/:namespace/:name/finetune/:id/cpu/:instance/usage", monitorHandler.CPUUsage)
		modelsMonitorGroup.GET("/:namespace/:name/finetune/:id/memory/:instance/usage", monitorHandler.MemoryUsage)
		modelsMonitorGroup.GET("/:namespace/:name/finetune/:id/request/:instance/count", monitorHandler.RequestCount)
		modelsMonitorGroup.GET("/:namespace/:name/finetune/:id/request/:instance/latency", monitorHandler.RequestLatency)
		// serverless monitor
		modelsMonitorGroup.GET("/:namespace/:name/serverless/:id/cpu/:instance/usage", monitorHandler.CPUUsage)
		modelsMonitorGroup.GET("/:namespace/:name/serverless/:id/memory/:instance/usage", monitorHandler.MemoryUsage)
		modelsMonitorGroup.GET("/:namespace/:name/serverless/:id/request/:instance/count", monitorHandler.RequestCount)
		modelsMonitorGroup.GET("/:namespace/:name/serverless/:id/request/:instance/latency", monitorHandler.RequestLatency)

		// evaluation monitor
		modelsMonitorGroup.GET("/evaluations/:id/cpu/:instance/usage", monitorHandler.CPUUsageEvaluation)
		modelsMonitorGroup.GET("/evaluations/:id/memory/:instance/usage", monitorHandler.MemoryUsageEvaluation)
	}

	modelsServerlessGroup := modelsGroup.Group("")
	{
		// deploy model as serverless
		modelsServerlessGroup.GET("/:namespace/:name/serverless", modelHandler.GetDeployServerless)
		modelsServerlessGroup.POST("/:namespace/:name/serverless", middlewareCollection.Auth.NeedAdmin, modelHandler.DeployServerless)
		modelsServerlessGroup.PUT("/:namespace/:name/serverless/:id/start", middlewareCollection.Auth.NeedAdmin, modelHandler.ServerlessStart)
		modelsServerlessGroup.PUT("/:namespace/:name/serverless/:id/stop", middlewareCollection.Auth.NeedAdmin, modelHandler.ServerlessStop)
		modelsServerlessGroup.GET("/:namespace/:name/serverless/:id", middlewareCollection.Auth.NeedAdmin, repoCommonHandler.ServerlessDetail)
		modelsServerlessGroup.GET("/:namespace/:name/serverless/:id/status", middlewareCollection.Auth.NeedAdmin, repoCommonHandler.ServerlessStatus)
		modelsServerlessGroup.GET("/:namespace/:name/serverless/:id/logs/:instance", middlewareCollection.Auth.NeedAdmin, repoCommonHandler.ServerlessLogs)
		modelsServerlessGroup.PUT("/:namespace/:name/serverless/:id", middlewareCollection.Auth.NeedAdmin, repoCommonHandler.ServerlessUpdate)
	}
}

func createDatasetRoutes(
	config *config.Config,
	apiGroup *gin.RouterGroup,
	middlewareCollection middleware.MiddlewareCollection,
	dsHandler *handler.DatasetHandler,
	repoCommonHandler *handler.RepoHandler,
) {
	datasetsGroup := apiGroup.Group("/datasets")
	// allow access without login
	datasetsGroup.GET("", dsHandler.Index)
	// must login
	datasetsGroup.Use(middleware.MustLogin(), middleware.RepoType(types.DatasetRepo), middlewareCollection.Repo.RepoExists)
	{
		datasetsGroup.POST("", middlewareCollection.Auth.NeedLogin, dsHandler.Create)
		datasetsGroup.PUT("/:namespace/:name", middlewareCollection.Auth.NeedLogin, dsHandler.Update)
		datasetsGroup.DELETE("/:namespace/:name", middlewareCollection.Auth.NeedLogin, dsHandler.Delete)
		datasetsGroup.GET("/:namespace/:name", dsHandler.Show)
		datasetsGroup.GET("/:namespace/:name/all_files", repoCommonHandler.AllFiles)
		datasetsGroup.GET("/:namespace/:name/relations", dsHandler.Relations)
	}

	// Models repo operation routes
	{
		datasetsGroup.GET("/:namespace/:name/branches", repoCommonHandler.Branches)
		datasetsGroup.GET("/:namespace/:name/tags", repoCommonHandler.Tags)
		datasetsGroup.POST("/:namespace/:name/preupload/:revision", repoCommonHandler.Preupload)
		// update tags of a certain category
		datasetsGroup.POST("/:namespace/:name/tags/:category", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateTags)
		datasetsGroup.GET("/:namespace/:name/last_commit", repoCommonHandler.LastCommit)
		datasetsGroup.GET("/:namespace/:name/commit/:commit_id", repoCommonHandler.CommitWithDiff)
		datasetsGroup.POST("/:namespace/:name/commit/:revision", repoCommonHandler.CommitFiles)
		datasetsGroup.GET("/:namespace/:name/remote_diff", repoCommonHandler.RemoteDiff)
		datasetsGroup.GET("/:namespace/:name/tree", repoCommonHandler.Tree)
		datasetsGroup.GET("/:namespace/:name/refs/:ref/tree/*path", repoCommonHandler.TreeV2)
		datasetsGroup.GET("/:namespace/:name/refs/:ref/logs_tree/*path", repoCommonHandler.LogsTree)
		datasetsGroup.GET("/:namespace/:name/commits", repoCommonHandler.Commits)
		datasetsGroup.POST("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.CreateFile)
		datasetsGroup.GET("/:namespace/:name/raw/*file_path", repoCommonHandler.FileRaw)
		datasetsGroup.GET("/:namespace/:name/blob/*file_path", repoCommonHandler.FileInfo)
		datasetsGroup.GET("/:namespace/:name/download/*file_path", repoCommonHandler.DownloadFile)
		datasetsGroup.GET("/:namespace/:name/resolve/*file_path", repoCommonHandler.ResolveDownload)
		datasetsGroup.PUT("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateFile)
		datasetsGroup.DELETE("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeleteFile)
		datasetsGroup.POST("/:namespace/:name/update_downloads", repoCommonHandler.UpdateDownloads)
		datasetsGroup.PUT("/:namespace/:name/incr_downloads", repoCommonHandler.IncrDownloads)
		datasetsGroup.POST("/:namespace/:name/upload_file", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UploadFile)
		datasetsGroup.POST("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.CreateMirror)
		datasetsGroup.GET("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.GetMirror)
		datasetsGroup.PUT("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateMirror)
		datasetsGroup.DELETE("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeleteMirror)
		datasetsGroup.POST("/:namespace/:name/mirror/sync", middlewareCollection.Auth.NeedLogin, repoCommonHandler.SyncMirror)

		// mirror from SaaS, only on-premises available
		if !config.Saas {
			datasetsGroup.POST("/:namespace/:name/mirror_from_saas", middlewareCollection.Auth.NeedLogin, repoCommonHandler.MirrorFromSaas)
		}
	}
}

func createCodeRoutes(
	config *config.Config,
	apiGroup *gin.RouterGroup,
	middlewareCollection middleware.MiddlewareCollection,
	codeHandler *handler.CodeHandler,
	repoCommonHandler *handler.RepoHandler,
) {
	codesGroup := apiGroup.Group("/codes")
	codesGroup.Use(middleware.RepoType(types.CodeRepo), middlewareCollection.Repo.RepoExists)
	{
		codesGroup.POST("", middlewareCollection.Auth.NeedLogin, codeHandler.Create)
		codesGroup.GET("", codeHandler.Index)
		codesGroup.PUT("/:namespace/:name", middlewareCollection.Auth.NeedLogin, codeHandler.Update)
		codesGroup.DELETE("/:namespace/:name", middlewareCollection.Auth.NeedLogin, codeHandler.Delete)
		codesGroup.GET("/:namespace/:name", codeHandler.Show)
		codesGroup.GET("/:namespace/:name/relations", codeHandler.Relations)
	}

	{
		codesGroup.GET("/:namespace/:name/branches", repoCommonHandler.Branches)
		codesGroup.GET("/:namespace/:name/tags", repoCommonHandler.Tags)
		codesGroup.POST("/:namespace/:name/preupload/:revision", repoCommonHandler.Preupload)
		// update tags of a certain category
		codesGroup.GET("/:namespace/:name/all_files", repoCommonHandler.AllFiles)
		codesGroup.POST("/:namespace/:name/tags/:category", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateTags)
		codesGroup.GET("/:namespace/:name/last_commit", repoCommonHandler.LastCommit)
		codesGroup.GET("/:namespace/:name/commit/:commit_id", repoCommonHandler.CommitWithDiff)
		codesGroup.POST("/:namespace/:name/commit/:revision", repoCommonHandler.CommitFiles)
		codesGroup.GET("/:namespace/:name/remote_diff", repoCommonHandler.RemoteDiff)
		codesGroup.GET("/:namespace/:name/tree", repoCommonHandler.Tree)
		codesGroup.GET("/:namespace/:name/refs/:ref/tree/*path", repoCommonHandler.TreeV2)
		codesGroup.GET("/:namespace/:name/refs/:ref/logs_tree/*path", repoCommonHandler.LogsTree)
		codesGroup.GET("/:namespace/:name/commits", repoCommonHandler.Commits)
		codesGroup.POST("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.CreateFile)
		codesGroup.GET("/:namespace/:name/raw/*file_path", repoCommonHandler.FileRaw)
		codesGroup.GET("/:namespace/:name/blob/*file_path", repoCommonHandler.FileInfo)
		codesGroup.GET("/:namespace/:name/download/*file_path", repoCommonHandler.DownloadFile)
		codesGroup.GET("/:namespace/:name/resolve/*file_path", repoCommonHandler.ResolveDownload)
		codesGroup.PUT("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateFile)
		codesGroup.DELETE("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeleteFile)
		codesGroup.POST("/:namespace/:name/update_downloads", repoCommonHandler.UpdateDownloads)
		codesGroup.PUT("/:namespace/:name/incr_downloads", repoCommonHandler.IncrDownloads)
		codesGroup.POST("/:namespace/:name/upload_file", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UploadFile)
		codesGroup.POST("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.CreateMirror)
		codesGroup.GET("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.GetMirror)
		codesGroup.PUT("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateMirror)
		codesGroup.DELETE("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeleteMirror)
		codesGroup.POST("/:namespace/:name/mirror/sync", middlewareCollection.Auth.NeedLogin, repoCommonHandler.SyncMirror)

		// mirror from SaaS, only on-premises available
		if !config.Saas {
			codesGroup.POST("/:namespace/:name/mirror_from_saas", middlewareCollection.Auth.NeedLogin, repoCommonHandler.MirrorFromSaas)
		}
	}
}

func createSpaceRoutes(config *config.Config,
	apiGroup *gin.RouterGroup,
	middlewareCollection middleware.MiddlewareCollection,
	spaceHandler *handler.SpaceHandler,
	repoCommonHandler *handler.RepoHandler,
	monitorHandler *handler.MonitorHandler) {
	spaces := apiGroup.Group("/spaces")
	spaces.Use(middleware.RepoType(types.SpaceRepo), middlewareCollection.Repo.RepoExists)
	{
		// list all spaces
		spaces.GET("", spaceHandler.Index)
		spaces.POST("", middlewareCollection.Auth.NeedLogin, spaceHandler.Create)
		// show a user or org's space
		spaces.GET("/:namespace/:name", middlewareCollection.Auth.NeedLogin, spaceHandler.Show)
		spaces.PUT("/:namespace/:name", middlewareCollection.Auth.NeedLogin, spaceHandler.Update)
		spaces.DELETE("/:namespace/:name", middlewareCollection.Auth.NeedLogin, spaceHandler.Delete)
		// depoly and start running the space
		spaces.POST("/:namespace/:name/run", middlewareCollection.Auth.NeedLogin, spaceHandler.Run)
		// wake a sleeping space
		spaces.POST("/:namespace/:name/wakeup", spaceHandler.Wakeup)
		// stop running space
		spaces.POST("/:namespace/:name/stop", middlewareCollection.Auth.NeedLogin, spaceHandler.Stop)
		// pull space running status
		spaces.GET("/:namespace/:name/status", spaceHandler.Status)
		// pull space building and running logs
		spaces.GET("/:namespace/:name/logs", spaceHandler.Logs)
		// call space webhook api
		spaces.POST("/:namespace/:name/webhook", nil)
	}
	{
		spaces.GET("/:namespace/:name/branches", repoCommonHandler.Branches)
		spaces.GET("/:namespace/:name/tags", repoCommonHandler.Tags)
		spaces.POST("/:namespace/:name/preupload/:revision", repoCommonHandler.Preupload)
		// update tags of a certain category
		spaces.POST("/:namespace/:name/tags/:category", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateTags)
		spaces.GET("/:namespace/:name/last_commit", repoCommonHandler.LastCommit)
		spaces.GET("/:namespace/:name/commit/:commit_id", repoCommonHandler.CommitWithDiff)
		spaces.POST("/:namespace/:name/commit/:revision", repoCommonHandler.CommitFiles)
		spaces.GET("/:namespace/:name/remote_diff", repoCommonHandler.RemoteDiff)
		spaces.GET("/:namespace/:name/tree", repoCommonHandler.Tree)
		spaces.GET("/:namespace/:name/refs/:ref/tree/*path", repoCommonHandler.TreeV2)
		spaces.GET("/:namespace/:name/refs/:ref/logs_tree/*path", repoCommonHandler.LogsTree)
		spaces.GET("/:namespace/:name/commits", repoCommonHandler.Commits)
		spaces.POST("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.CreateFile)
		spaces.GET("/:namespace/:name/raw/*file_path", repoCommonHandler.FileRaw)
		spaces.GET("/:namespace/:name/blob/*file_path", repoCommonHandler.FileInfo)
		spaces.GET("/:namespace/:name/download/*file_path", repoCommonHandler.DownloadFile)
		spaces.GET("/:namespace/:name/resolve/*file_path", repoCommonHandler.ResolveDownload)
		spaces.PUT("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateFile)
		spaces.DELETE("/:namespace/:name/raw/*file_path", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeleteFile)
		spaces.POST("/:namespace/:name/update_downloads", repoCommonHandler.UpdateDownloads)
		spaces.PUT("/:namespace/:name/incr_downloads", repoCommonHandler.IncrDownloads)
		spaces.POST("/:namespace/:name/upload_file", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UploadFile)
		spaces.POST("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.CreateMirror)
		spaces.GET("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.GetMirror)
		spaces.PUT("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateMirror)
		spaces.DELETE("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeleteMirror)
		spaces.POST("/:namespace/:name/mirror/sync", middlewareCollection.Auth.NeedLogin, repoCommonHandler.SyncMirror)

		// mirror from SaaS, only on-premises available
		if !config.Saas {
			spaces.POST("/:namespace/:name/mirror_from_saas", middlewareCollection.Auth.NeedLogin, repoCommonHandler.MirrorFromSaas)
		}
		spaces.GET("/:namespace/:name/run", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeployList)
		spaces.GET("/:namespace/:name/run/:id", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeployDetail)
		spaces.GET("/:namespace/:name/run/:id/status", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeployStatus)
		spaces.GET("/:namespace/:name/run/:id/logs/:instance", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeployInstanceLogs)
	}
	spaceMonitorGroup := spaces.Group("")
	spaceMonitorGroup.Use(middlewareCollection.Auth.NeedLogin)
	{
		// space monitor
		spaceMonitorGroup.GET("/:namespace/:name/run/:id/cpu/:instance/usage",
			monitorHandler.CPUUsage)
		spaceMonitorGroup.GET("/:namespace/:name/run/:id/memory/:instance/usage",
			monitorHandler.MemoryUsage)
		spaceMonitorGroup.GET("/:namespace/:name/run/:id/request/:instance/count",
			monitorHandler.RequestCount)
		spaceMonitorGroup.GET("/:namespace/:name/run/:id/request/:instance/latency",
			monitorHandler.RequestLatency)
	}
}

func createUserRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, userProxyHandler *handler.InternalServiceProxyHandler, userHandler *handler.UserHandler) {
	// deprecated
	{
		apiGroup.POST("/users", userProxyHandler.ProxyToApi("/api/v1/user"))
		apiGroup.PUT("/users/:username", userProxyHandler.ProxyToApi("/api/v1/user/%v", "username"))
	}

	{
		apiGroup.POST("/user", userProxyHandler.Proxy)
		apiGroup.GET("/user/:username", userProxyHandler.Proxy)
		apiGroup.PUT("/user/:username", userProxyHandler.Proxy)
		apiGroup.DELETE("/user/:username", userProxyHandler.Proxy)
	}

	{
		// User models
		apiGroup.GET("/user/:username/models", userHandler.Models)
		// User datasets
		apiGroup.GET("/user/:username/datasets", userHandler.Datasets)
		apiGroup.GET("/user/:username/codes", userHandler.Codes)
		apiGroup.GET("/user/:username/spaces", userHandler.Spaces)
		apiGroup.GET("/user/:username/prompts", userHandler.Prompts)
		apiGroup.GET("/user/:username/mcps", userHandler.MCPServers)
	}

	{
		// User likes
		apiGroup.PUT("/user/:username/likes/:repo_id", middlewareCollection.Auth.NeedLogin, userHandler.LikesAdd)
		apiGroup.DELETE("/user/:username/likes/:repo_id", middlewareCollection.Auth.NeedLogin, userHandler.LikesDelete)
		apiGroup.GET("/user/:username/likes/spaces", middlewareCollection.Auth.NeedLogin, userHandler.LikesSpaces)
		apiGroup.GET("/user/:username/likes/codes", middlewareCollection.Auth.NeedLogin, userHandler.LikesCodes)
		apiGroup.GET("/user/:username/likes/models", middlewareCollection.Auth.NeedLogin, userHandler.LikesModels)
		apiGroup.GET("/user/:username/likes/datasets", middlewareCollection.Auth.NeedLogin, userHandler.LikesDatasets)
		apiGroup.GET("/user/:username/likes/mcps", middlewareCollection.Auth.NeedLogin, userHandler.LikesMCPServers)
	}

	{
		apiGroup.GET("/user/:username/run/:repo_type", middlewareCollection.Auth.UserMatch, userHandler.GetRunDeploys)
		apiGroup.GET("/user/:username/finetune/instances", middlewareCollection.Auth.UserMatch, userHandler.GetFinetuneInstances)
		// User evaluations
		apiGroup.GET("/user/:username/evaluations", middlewareCollection.Auth.NeedLogin, userHandler.GetEvaluations)
	}

	// User collection
	apiGroup.GET("/user/:username/collections", userHandler.UserCollections)
	apiGroup.GET("/user/:username/likes/collections", middlewareCollection.Auth.NeedLogin, userHandler.LikesCollections)
	apiGroup.PUT("/user/:username/likes/collections/:id", middlewareCollection.Auth.NeedLogin, userHandler.LikeCollection)
	apiGroup.DELETE("/user/:username/likes/collections/:id", middlewareCollection.Auth.NeedLogin, userHandler.UnLikeCollection)
	// user owned tokens
	apiGroup.GET("/user/:username/tokens", userProxyHandler.ProxyToApi("/api/v1/user/%s/tokens", "username"))

	// serverless list
	apiGroup.GET("/user/:username/run/serverless", middlewareCollection.Auth.NeedAdmin, userHandler.GetRunServerless)
}

func createRuntimeFrameworkRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, modelHandler *handler.ModelHandler, runtimeArchHandler *handler.RuntimeArchitectureHandler, repoCommonHandler *handler.RepoHandler) {
	needAdmin := middlewareCollection.Auth.NeedAdmin
	runtimeFramework := apiGroup.Group("/runtime_framework")
	{
		runtimeFramework.GET("/:id/models", middlewareCollection.Auth.NeedLogin, modelHandler.ListByRuntimeFrameworkID)
		runtimeFramework.GET("", middlewareCollection.Auth.NeedLogin, modelHandler.ListAllRuntimeFramework)
		runtimeFramework.POST("", middlewareCollection.Auth.NeedLogin, repoCommonHandler.RuntimeFrameworkCreate)
		runtimeFramework.PUT("/:id", middlewareCollection.Auth.NeedLogin, repoCommonHandler.RuntimeFrameworkUpdate)
		runtimeFramework.DELETE("/:id", middlewareCollection.Auth.NeedLogin, repoCommonHandler.RuntimeFrameworkDelete)
		runtimeFramework.PUT("/:id/models", middlewareCollection.Auth.NeedAdmin, modelHandler.UpdateModelRuntimeFrameworks)
		runtimeFramework.DELETE("/:id/models", middlewareCollection.Auth.NeedAdmin, modelHandler.DeleteModelRuntimeFrameworks)
		runtimeFramework.GET("/models", middlewareCollection.Auth.NeedLogin, modelHandler.ListModelsOfRuntimeFrameworks)

		runtimeFramework.GET("/:id/architecture", needAdmin, runtimeArchHandler.ListByRuntimeFrameworkID)
		runtimeFramework.PUT("/:id/architecture", needAdmin, runtimeArchHandler.UpdateArchitecture)
		runtimeFramework.DELETE("/:id/architecture", needAdmin, runtimeArchHandler.DeleteArchitecture)
		runtimeFramework.POST("/scan", needAdmin, runtimeArchHandler.ScanArchitecture)
		runtimeFramework.POST("/:namespace/:name/scan", middlewareCollection.Auth.NeedLogin, runtimeArchHandler.ScanArchForSingleModel)
	}
}

func createAccountRoutes(
	apiGroup *gin.RouterGroup,
	middlewareCollection middleware.MiddlewareCollection,
	accountingHandler *handler.AccountingHandler,
) {
	accountingGroup := apiGroup.Group("/accounting")
	{
		meterGroup := accountingGroup.Group("/metering")
		{
			meterGroup.GET("/:id/statements", middlewareCollection.Auth.NeedLogin, accountingHandler.QueryMeteringStatementByUserID)
		}
	}
}

func createMappingRoutes(
	r *gin.Engine,
	group string,
	middlewareCollection middleware.MiddlewareCollection,
	hfdsHandler *handler.HFDatasetHandler,
	repoCommonHandler *handler.RepoHandler,
	modelHandler *handler.ModelHandler,
	userHandler *handler.UserHandler,
	gitHTTPHandler *handler.GitHTTPHandler,
) {
	// Huggingface SDK routes
	hfGroup := r.Group(group)
	{
		hfGroup.GET("/:namespace/:name/resolve/:branch/*file_path", middleware.RepoMapping(types.ModelRepo), repoCommonHandler.SDKDownload)
		hfGroup.HEAD("/:namespace/:name/resolve/:branch/*file_path", middleware.RepoMapping(types.ModelRepo), repoCommonHandler.HeadSDKDownload)
		lfsGroup := hfGroup.Group("/:namespace/:name/info/lfs")
		{
			objectsGroup := lfsGroup.Group("/objects")
			{
				objectsGroup.POST("/batch", gitHTTPHandler.LfsBatchHF)
				objectsGroup.PUT("/:oid/:size", gitHTTPHandler.LfsUpload)
				lfsGroup.GET("/:oid", gitHTTPHandler.LfsDownload)
			}
			lfsGroup.POST("/verify", gitHTTPHandler.LfsVerify)
		}
		hfdsFileGroup := hfGroup.Group("/datasets")
		{
			hfdsFileGroup.GET("/:namespace/:name/resolve/:branch/*file_path", middleware.RepoMapping(types.DatasetRepo), repoCommonHandler.SDKDownload)
			hfdsFileGroup.HEAD("/:namespace/:name/resolve/:branch/*file_path", middleware.RepoMapping(types.DatasetRepo), repoCommonHandler.HeadSDKDownload)
		}
		hfSpaceFileGroup := hfGroup.Group("/spaces")
		{
			hfSpaceFileGroup.GET("/:namespace/:name/resolve/:branch/*file_path", middleware.RepoMapping(types.SpaceRepo), repoCommonHandler.SDKDownload)
			hfSpaceFileGroup.HEAD("/:namespace/:name/resolve/:branch/*file_path", middleware.RepoMapping(types.SpaceRepo), repoCommonHandler.HeadSDKDownload)
		}
		hfAPIGroup := hfGroup.Group("/api")
		{
			hfAPIGroup.GET("/whoami-v2", middlewareCollection.Auth.NeedLogin, userHandler.UserPermission)
			hfModelAPIGroup := hfAPIGroup.Group("/models")
			{
				// compatible with HF model info api, used for sdk like this:  huggingface_hub.model_info(repo_id, revision)
				hfModelAPIGroup.GET("/:namespace/:name/revision/:ref", middleware.RepoMapping(types.ModelRepo), modelHandler.SDKModelInfo)
				hfModelAPIGroup.GET("/:namespace/:name", middleware.RepoMapping(types.ModelRepo), modelHandler.SDKModelInfo)
				hfModelAPIGroup.POST("/:namespace/:name/preupload/:revision", middleware.RepoMapping(types.ModelRepo), repoCommonHandler.PreuploadHF)
				hfModelAPIGroup.POST("/:namespace/:name/commit/:revision", middleware.RepoMapping(types.ModelRepo), repoCommonHandler.CommitFilesHF)
			}
			hfDSAPIGroup := hfAPIGroup.Group("/datasets")
			{
				// compatible with HF dataset info api, used for sdk like this: huggingface_hub.dataset_info(repo_id, revision)
				hfDSAPIGroup.GET("/:namespace/:name/revision/:ref", middleware.RepoMapping(types.DatasetRepo), repoCommonHandler.SDKListFiles)
				hfDSAPIGroup.GET("/:namespace/:name", middleware.RepoMapping(types.DatasetRepo), repoCommonHandler.SDKListFiles)
				hfDSAPIGroup.POST("/:namespace/:name/paths-info/:ref", hfdsHandler.DatasetPathsInfo)
				hfDSAPIGroup.GET("/:namespace/:name/tree/:ref/*path_in_repo", hfdsHandler.DatasetTree)
				hfDSAPIGroup.GET("/:namespace/:name/resolve/:ref/.huggingface.yaml", hfdsHandler.HandleHFYaml)
			}
			hfSpaceAPIGroup := hfAPIGroup.Group("/spaces")
			{
				hfSpaceAPIGroup.GET("/:namespace/:name/revision/:ref", middleware.RepoMapping(types.SpaceRepo), repoCommonHandler.SDKListFiles)
				hfSpaceAPIGroup.GET("/:namespace/:name", middleware.RepoMapping(types.SpaceRepo), repoCommonHandler.SDKListFiles)
			}
			hfReposAPIGroup := hfAPIGroup.Group("/repos")
			{
				hfReposAPIGroup.POST("/create", middlewareCollection.Auth.NeedLogin, repoCommonHandler.CreateRepo)
			}
			hfAPIGroup.POST("/validate-yaml", middlewareCollection.Auth.NeedLogin, repoCommonHandler.ValidateYaml)
		}
	}
}

func createDiscussionRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, discussionHandler *handler.DiscussionHandler) {
	apiGroup.POST("/:repo_type/:namespace/:name/discussions", middlewareCollection.Auth.NeedLogin, discussionHandler.CreateRepoDiscussion)
	apiGroup.GET("/:repo_type/:namespace/:name/discussions", discussionHandler.ListRepoDiscussions)
	apiGroup.GET("/discussions/:id", discussionHandler.ShowDiscussion)
	apiGroup.PUT("/discussions/:id", middlewareCollection.Auth.NeedLogin, discussionHandler.UpdateDiscussion)
	apiGroup.DELETE("/discussions/:id", middlewareCollection.Auth.NeedLogin, discussionHandler.DeleteDiscussion)
	apiGroup.POST("/discussions/:id/comments", middlewareCollection.Auth.NeedLogin, discussionHandler.CreateDiscussionComment)
	apiGroup.GET("/discussions/:id/comments", discussionHandler.ListDiscussionComments)
	apiGroup.PUT("/discussions/:id/comments/:comment_id", middlewareCollection.Auth.NeedLogin, discussionHandler.UpdateComment)
	apiGroup.DELETE("/discussions/:id/comments/:comment_id", middlewareCollection.Auth.NeedLogin, discussionHandler.DeleteComment)
}

func createPromptRoutes(
	apiGroup *gin.RouterGroup,
	middlewareCollection middleware.MiddlewareCollection,
	promptHandler *handler.PromptHandler,
	repoCommonHandler *handler.RepoHandler,
) {
	promptGrp := apiGroup.Group("/prompts")
	promptGrp.Use(middleware.RepoType(types.PromptRepo), middlewareCollection.Repo.RepoExists)
	{
		promptGrp.GET("", promptHandler.Index)
		promptGrp.GET("/:namespace/:name", promptHandler.ListPrompt)
		promptGrp.GET("/:namespace/:name/relations", promptHandler.Relations)
		promptGrp.GET("/:namespace/:name/prompt/view/*file_path", promptHandler.GetPrompt)
		promptGrp.POST("/:namespace/:name/prompt/record", middlewareCollection.Auth.NeedLogin, promptHandler.CreatePrompt)
		promptGrp.PUT("/:namespace/:name/prompt/record/*file_path", middlewareCollection.Auth.NeedLogin, promptHandler.UpdatePrompt)
		promptGrp.DELETE("/:namespace/:name/prompt/record/*file_path", middlewareCollection.Auth.NeedLogin, promptHandler.DeletePrompt)

		promptGrp.PUT("/:namespace/:name/relations", middlewareCollection.Auth.NeedLogin, promptHandler.SetRelations)
		promptGrp.POST("/:namespace/:name/relations/model", middlewareCollection.Auth.NeedAdmin, promptHandler.AddModelRelation)
		promptGrp.DELETE("/:namespace/:name/relations/model", middlewareCollection.Auth.NeedAdmin, promptHandler.DelModelRelation)

		promptGrp.POST("", middlewareCollection.Auth.NeedLogin, promptHandler.Create)
		promptGrp.PUT("/:namespace/:name", middlewareCollection.Auth.NeedLogin, promptHandler.Update)
		promptGrp.DELETE("/:namespace/:name", middlewareCollection.Auth.NeedLogin, promptHandler.Delete)

		promptGrp.GET("/:namespace/:name/branches", promptHandler.Branches)
		promptGrp.GET("/:namespace/:name/tags", promptHandler.Tags)
		promptGrp.POST("/:namespace/:name/tags/:category", middlewareCollection.Auth.NeedLogin, promptHandler.UpdateTags)
		promptGrp.POST("/:namespace/:name/update_downloads", promptHandler.UpdateDownloads)
	}
	{
		promptGrp.GET("/:namespace/:name/all_files", repoCommonHandler.AllFiles)
		promptGrp.POST("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.CreateMirror)
		promptGrp.GET("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.GetMirror)
		promptGrp.PUT("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.UpdateMirror)
		promptGrp.DELETE("/:namespace/:name/mirror", middlewareCollection.Auth.NeedLogin, repoCommonHandler.DeleteMirror)
		promptGrp.POST("/:namespace/:name/mirror/sync", middlewareCollection.Auth.NeedLogin, repoCommonHandler.SyncMirror)
		promptGrp.POST("/:namespace/:name/mirror_from_saas", middlewareCollection.Auth.NeedLogin, repoCommonHandler.MirrorFromSaas)
	}
}

func createDataflowRoutes(apiGroup *gin.RouterGroup, dataflowHandler *handler.DataflowProxyHandler) {
	dataflowGrp := apiGroup.Group("/dataflow")
	dataflowGrp.Use(middleware.MustLogin())
	dataflowGrp.Any("/*any", dataflowHandler.Proxy)
}

func createTagsRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, tagHandler *handler.TagsHandler) {
	tagsGrp := apiGroup.Group("/tags")
	{
		// TODO: Remove admin check in tagComponent
		categoryGrp := tagsGrp.Group("/categories")
		{
			categoryGrp.GET("", tagHandler.AllCategories)
			categoryGrp.POST("", middlewareCollection.Auth.NeedAdmin, tagHandler.CreateCategory)
			categoryGrp.PUT("/:id", middlewareCollection.Auth.NeedAdmin, tagHandler.UpdateCategory)
			categoryGrp.DELETE("/:id", middlewareCollection.Auth.NeedAdmin, tagHandler.DeleteCategory)
		}
		tagsGrp.GET("", tagHandler.AllTags)
		tagsGrp.POST("", middlewareCollection.Auth.NeedAdmin, tagHandler.CreateTag)
		tagsGrp.GET("/:id", middlewareCollection.Auth.NeedAdmin, tagHandler.GetTagByID)
		tagsGrp.PUT("/:id", middlewareCollection.Auth.NeedAdmin, tagHandler.UpdateTag)
		tagsGrp.DELETE("/:id", middlewareCollection.Auth.NeedAdmin, tagHandler.DeleteTag)
	}
}

func createDataViewerRoutes(
	apiGroup *gin.RouterGroup,
	middlewareCollection middleware.MiddlewareCollection,
	dsViewerHandler *handler.InternalServiceProxyHandler,
) {
	datasetRepoGrp := apiGroup.Group("/datasets/:namespace/:name")
	datasetRepoGrp.Use(middlewareCollection.Auth.NeedLogin)
	fileViewerGrp := datasetRepoGrp.Group("/viewer")
	{
		fileViewerGrp.Any("/*any", dsViewerHandler.Proxy)
	}
	dataViewerGrp := datasetRepoGrp.Group("/dataviewer")
	{
		dataViewerGrp.Any("/*any", dsViewerHandler.Proxy)
	}
}

func createSpaceTemplateRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, templateHandler *handler.SpaceTemplateHandler) {
	spaceTemplateGrp := apiGroup.Group("/space_templates")
	{
		spaceTemplateGrp.GET("", middlewareCollection.Auth.NeedAdmin, templateHandler.Index)
		spaceTemplateGrp.POST("", middlewareCollection.Auth.NeedAdmin, templateHandler.Create)
		spaceTemplateGrp.PUT("/:id", middlewareCollection.Auth.NeedAdmin, templateHandler.Update)
		spaceTemplateGrp.DELETE("/:id", middlewareCollection.Auth.NeedAdmin, templateHandler.Delete)
		spaceTemplateGrp.GET("/:type", templateHandler.List)
	}
}

func createOrgRoutes(apiGroup *gin.RouterGroup, userProxyHandler *handler.InternalServiceProxyHandler, orgHandler *handler.OrganizationHandler) {
	{
		apiGroup.GET("/organizations", userProxyHandler.Proxy)
		apiGroup.POST("/organizations", userProxyHandler.Proxy)
		apiGroup.GET("/organization/:namespace", userProxyHandler.ProxyToApi("/api/v1/organization/%s", "namespace"))
		apiGroup.PUT("/organization/:namespace", userProxyHandler.ProxyToApi("/api/v1/organization/%s", "namespace"))
		apiGroup.DELETE("/organization/:namespace", userProxyHandler.ProxyToApi("/api/v1/organization/%s", "namespace"))
		// Organization assets
		apiGroup.GET("/organization/:namespace/models", orgHandler.Models)
		apiGroup.GET("/organization/:namespace/datasets", orgHandler.Datasets)
		apiGroup.GET("/organization/:namespace/codes", orgHandler.Codes)
		apiGroup.GET("/organization/:namespace/spaces", orgHandler.Spaces)
		apiGroup.GET("/organization/:namespace/collections", orgHandler.Collections)
		apiGroup.GET("/organization/:namespace/prompts", orgHandler.Prompts)
		apiGroup.GET("/organization/:namespace/mcps", orgHandler.MCPServers)
	}

	{
		apiGroup.GET("/organization/:namespace/members", userProxyHandler.ProxyToApi("/api/v1/organization/%s/members", "namespace"))
		apiGroup.POST("/organization/:namespace/members", userProxyHandler.ProxyToApi("/api/v1/organization/%s/members", "namespace"))
		apiGroup.GET("/organization/:namespace/members/:username", userProxyHandler.ProxyToApi("/api/v1/organization/%s/members/%s", "namespace", "username"))
		apiGroup.PUT("/organization/:namespace/members/:username", userProxyHandler.ProxyToApi("/api/v1/organization/%s/members/%s", "namespace", "username"))
		apiGroup.DELETE("/organization/:namespace/members/:username", userProxyHandler.ProxyToApi("/api/v1/organization/%s/members/%s", "namespace", "username"))
	}
}

func createNotificationRoutes(config *config.Config, apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection) error {
	notificationProxyHandler, err := handler.NewInternalServiceProxyHandler(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port))
	if err != nil {
		return fmt.Errorf("error creating notifier controller: %w", err)
	}

	notificationsGroup := apiGroup.Group("/notifications")
	notificationsGroup.Use(middlewareCollection.Auth.NeedLogin)
	{
		notificationsGroup.GET("/count", notificationProxyHandler.Proxy)
		notificationsGroup.GET("", notificationProxyHandler.Proxy)
		notificationsGroup.POST("", notificationProxyHandler.Proxy)
		notificationsGroup.PUT("/read", notificationProxyHandler.Proxy)
		notificationsGroup.PUT("/setting", notificationProxyHandler.Proxy)
		notificationsGroup.GET("/setting", notificationProxyHandler.Proxy)
		notificationsGroup.GET("/poll/:limit", notificationProxyHandler.Proxy)
		notificationsGroup.GET("/message-types", notificationProxyHandler.Proxy)
		notificationsGroup.PUT("/msg-task", middlewareCollection.Auth.NeedAdmin, notificationProxyHandler.Proxy)
	}

	return nil
}

func createLfsSyncRoutes(apiGroup *gin.RouterGroup, middlewareCollection middleware.MiddlewareCollection, lfsSyncHandler *handler.InternalServiceProxyHandler) {
	lfsSyncGrp := apiGroup.Group("/lfs_sync")
	{
		lfsSyncGrp.POST("/stop_worker_by_id", middlewareCollection.Auth.NeedAdmin, lfsSyncHandler.ProxyToApi("/api/v1/lfs_sync/stop_worker_by_id"))
		lfsSyncGrp.POST("/sync_now", middlewareCollection.Auth.NeedAdmin, lfsSyncHandler.ProxyToApi("/api/v1/lfs_sync/sync_now"))
		lfsSyncGrp.POST("/cancel", middlewareCollection.Auth.NeedAdmin, lfsSyncHandler.ProxyToApi("/api/v1/lfs_sync/cancel"))
	}
}
