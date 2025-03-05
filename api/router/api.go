package router

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	cache "github.com/chenyahui/gin-cache"
	"github.com/chenyahui/gin-cache/persist"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/handler/callback"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/builder/instrumentation"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror"
)

type UserProxyHandler handler.InternalServiceProxyHandler
type DatasetViewerPeoxyHandler handler.InternalServiceProxyHandler

func newUserProxyHandler(config *config.Config) (*UserProxyHandler, error) {

	h, err := handler.NewInternalServiceProxyHandler(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port))
	return (*UserProxyHandler)(h), err
}

func newDatasetViewerProxyHandler(config *config.Config) (*DatasetViewerPeoxyHandler, error) {
	dataViewerAddr := fmt.Sprintf("%s:%d", config.DataViewer.Host, config.DataViewer.Port)
	h, err := handler.NewInternalServiceProxyHandler(dataViewerAddr)
	return (*DatasetViewerPeoxyHandler)(h), err
}

func newMemoryStore() *persist.MemoryStore {
	return persist.NewMemoryStore(1 * time.Minute)
}

type BaseServer struct {
	Config                   *config.Config
	Middleware               *middleware.Middleware
	GitHTTPHandler           *handler.GitHTTPHandler
	UserHandler              *handler.UserHandler
	OrgHandler               *handler.OrganizationHandler
	RepoCommonHandler        *handler.RepoHandler
	ModelHandler             *handler.ModelHandler
	DsHandler                *handler.DatasetHandler
	MirrorHandler            *handler.MirrorHandler
	HfdsHandler              *handler.HFDatasetHandler
	ListHandler              *handler.ListHandler
	EvaluationHandler        *handler.EvaluationHandler
	CodeHandler              *handler.CodeHandler
	SpaceHandler             *handler.SpaceHandler
	SpaceResourceHandler     *handler.SpaceResourceHandler
	SpaceSdkHandler          *handler.SpaceSdkHandler
	UserProxyHandler         *handler.InternalServiceProxyHandler
	SshKeyHandler            *handler.SSHKeyHandler
	TagCtrl                  *handler.TagsHandler
	CallbackCtrl             *callback.GitCallbackHandler
	SensitiveCtrl            *handler.SensitiveHandler
	MsHandler                *handler.MirrorSourceHandler
	CollectionHandler        *handler.CollectionHandler
	ClusterHandler           *handler.ClusterHandler
	EventHandler             *handler.EventHandler
	BroadcastHandler         *handler.BroadcastHandler
	RuntimeArchHandler       *handler.RuntimeArchitectureHandler
	SyncHandler              *handler.SyncHandler
	SyncClientSettingHandler *handler.SyncClientSettingHandler
	MeteringHandler          *handler.AccountingHandler
	RecomHandler             *handler.RecomHandler
	TelemetryHandler         *handler.TelemetryHandler
	InternalHandler          *handler.InternalHandler
	DiscussionHandler        *handler.DiscussionHandler
	PromptHandler            *handler.PromptHandler
	DsViewerHandler          *handler.InternalServiceProxyHandler
	PaymentProxyHandler      *handler.InternalServiceProxyHandler
	MemoryStore              *persist.MemoryStore
	UserServiceClient        rpc.UserSvcClient
	Engine                   *gin.Engine
}

func NewBaseServer(
	config *config.Config,
	middleware *middleware.Middleware,
	gitHTTPHandler *handler.GitHTTPHandler,
	userHandler *handler.UserHandler,
	orgHandler *handler.OrganizationHandler,
	repoCommonHandler *handler.RepoHandler,
	modelHandler *handler.ModelHandler,
	dsHandler *handler.DatasetHandler,
	mirrorHandler *handler.MirrorHandler,
	hfdsHandler *handler.HFDatasetHandler,
	listHandler *handler.ListHandler,
	evaluationHandler *handler.EvaluationHandler,
	codeHandler *handler.CodeHandler,
	spaceHandler *handler.SpaceHandler,
	spaceResourceHandler *handler.SpaceResourceHandler,
	spaceSdkHandler *handler.SpaceSdkHandler,
	userProxyHandler *UserProxyHandler,
	datasetViewerProxyHandler *DatasetViewerPeoxyHandler,
	sshKeyHandler *handler.SSHKeyHandler,
	tagCtrl *handler.TagsHandler,
	callbackCtrl *callback.GitCallbackHandler,
	sensitiveCtrl *handler.SensitiveHandler,
	msHandler *handler.MirrorSourceHandler,
	collectionHandler *handler.CollectionHandler,
	clusterHandler *handler.ClusterHandler,
	eventHandler *handler.EventHandler,
	broadcastHandler *handler.BroadcastHandler,
	runtimeArchHandler *handler.RuntimeArchitectureHandler,
	syncHandler *handler.SyncHandler,
	syncClientSettingHandler *handler.SyncClientSettingHandler,
	meteringHandler *handler.AccountingHandler,
	recomHandler *handler.RecomHandler,
	telemetryHandler *handler.TelemetryHandler,
	internalHandler *handler.InternalHandler,
	discussionHandler *handler.DiscussionHandler,
	promptHandler *handler.PromptHandler,
	memoryStore *persist.MemoryStore,
	userServiceClient rpc.UserSvcClient,

) (*BaseServer, error) {
	server := &BaseServer{
		Config:                   config,
		Middleware:               middleware,
		GitHTTPHandler:           gitHTTPHandler,
		UserHandler:              userHandler,
		OrgHandler:               orgHandler,
		RepoCommonHandler:        repoCommonHandler,
		ModelHandler:             modelHandler,
		DsHandler:                dsHandler,
		MirrorHandler:            mirrorHandler,
		HfdsHandler:              hfdsHandler,
		ListHandler:              listHandler,
		EvaluationHandler:        evaluationHandler,
		CodeHandler:              codeHandler,
		SpaceHandler:             spaceHandler,
		SpaceResourceHandler:     spaceResourceHandler,
		SpaceSdkHandler:          spaceSdkHandler,
		UserProxyHandler:         (*handler.InternalServiceProxyHandler)(userProxyHandler),
		SshKeyHandler:            sshKeyHandler,
		TagCtrl:                  tagCtrl,
		CallbackCtrl:             callbackCtrl,
		SensitiveCtrl:            sensitiveCtrl,
		MsHandler:                msHandler,
		CollectionHandler:        collectionHandler,
		ClusterHandler:           clusterHandler,
		EventHandler:             eventHandler,
		BroadcastHandler:         broadcastHandler,
		RuntimeArchHandler:       runtimeArchHandler,
		SyncHandler:              syncHandler,
		SyncClientSettingHandler: syncClientSettingHandler,
		MeteringHandler:          meteringHandler,
		RecomHandler:             recomHandler,
		TelemetryHandler:         telemetryHandler,
		InternalHandler:          internalHandler,
		DiscussionHandler:        discussionHandler,
		PromptHandler:            promptHandler,
		MemoryStore:              memoryStore,
		UserServiceClient:        userServiceClient,
		DsViewerHandler:          (*handler.InternalServiceProxyHandler)(datasetViewerProxyHandler),
	}
	return server, nil
}

func RunServer(config *config.Config, enableSwagger bool) {
	stopOtel, err := instrumentation.SetupOTelSDK(context.Background(), config, "csghub-api")
	if err != nil {
		panic(err)
	}
	slog.Info("init gin http router")
	srv, err := InitializeServer(config)
	if err != nil {
		panic(err)
	}
	err = srv.RegisterRoutes(enableSwagger)
	if err != nil {
		panic(err)
	}
	slog.Info("csghub service is running", slog.Any("port", config.APIServer.Port))
	server := httpbase.NewGracefulServer(
		httpbase.GraceServerOpt{
			Port: config.APIServer.Port,
		},
		srv.Engine,
	)
	// Initialize mirror service
	mirrorService, err := mirror.NewMirrorPriorityQueue(config)
	if err != nil {
		panic(fmt.Errorf("failed to init mirror service: %w", err))
	}

	if config.MirrorServer.Enable && config.GitServer.Type == types.GitServerTypeGitaly {
		mirrorService.EnqueueMirrorTasks()
	}

	server.Run()
	_ = stopOtel(context.Background())
	temporal.Stop()
}

func (s *BaseServer) GetEngine() *gin.Engine {
	return s.Engine
}

func (s *BaseServer) RegisterRoutes(enableSwagger bool) error {
	r := gin.New()
	s.Engine = r
	if s.Config.Instrumentation.OTLPEndpoint != "" {
		r.Use(otelgin.Middleware("csghub-server"))
	}

	r.Use(cors.New(cors.Config{
		AllowCredentials: true,
		AllowHeaders:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowAllOrigins:  true,
	}))
	r.Use(gin.Recovery())
	r.Use(s.Middleware.Log())

	//add router for golang pprof
	debugGroup := r.Group("/debug", s.Middleware.NeedAPIKey())
	pprof.RouteRegister(debugGroup, "pprof")

	gitHTTP := r.Group("/:repo_type/:namespace/:name")
	gitHTTP.Use(s.Middleware.GitHTTPParamMiddleware())
	gitHTTP.Use(s.Middleware.GetCurrentUserFromHeader())
	{
		gitHTTP.GET("/info/refs", s.GitHTTPHandler.InfoRefs)
		gitHTTP.POST("/git-upload-pack", s.Middleware.ContentEncoding(), s.GitHTTPHandler.GitUploadPack)
		gitHTTP.POST("/git-receive-pack", s.Middleware.ContentEncoding(), s.GitHTTPHandler.GitReceivePack)

		lfsGroup := gitHTTP.Group("/info/lfs")
		{
			objectsGroup := lfsGroup.Group("/objects")
			{
				objectsGroup.POST("/batch", s.GitHTTPHandler.LfsBatch)
				objectsGroup.PUT("/:oid/:size", s.GitHTTPHandler.LfsUpload)
				lfsGroup.GET("/:oid", s.GitHTTPHandler.LfsDownload)
			}
			lfsGroup.POST("/verify", s.GitHTTPHandler.LfsVerify)

			locksGroup := lfsGroup.Group("/locks")
			{
				locksGroup.GET("", s.GitHTTPHandler.ListLocks)
				locksGroup.POST("", s.GitHTTPHandler.CreateLock)
				locksGroup.POST("/verify", s.GitHTTPHandler.VerifyLock)
				locksGroup.POST("/:lid/unlock", s.GitHTTPHandler.UnLock)
			}

		}

	}
	r.Use(s.Middleware.Authenticator())

	if enableSwagger {
		r.GET("/api/v1/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	//create routes for hf
	createMappingRoutes(r, "/hf", s.HfdsHandler, s.RepoCommonHandler, s.ModelHandler, s.UserHandler, s.Middleware)
	//create routes for ms
	createMappingRoutes(r, "/ms", s.HfdsHandler, s.RepoCommonHandler, s.ModelHandler, s.UserHandler, s.Middleware)
	//create routes for csg
	createMappingRoutes(r, "/csg", s.HfdsHandler, s.RepoCommonHandler, s.ModelHandler, s.UserHandler, s.Middleware)

	apiGroup := r.Group("/api/v1")

	// List trending models and datasets routes
	{
		apiGroup.POST("/list/models_by_path", cache.CacheByRequestURI(s.MemoryStore, 1*time.Minute), s.ListHandler.ListModelsByPath)
		apiGroup.POST("/list/datasets_by_path", cache.CacheByRequestURI(s.MemoryStore, 1*time.Minute), s.ListHandler.ListDatasetsByPath)
		apiGroup.POST("/list/spaces_by_path", cache.CacheByRequestURI(s.MemoryStore, 1*time.Minute), s.ListHandler.ListSpacesByPath)
	}

	//evaluation handler
	createEvaluationRoutes(apiGroup, s.EvaluationHandler)

	// Model routes
	createModelRoutes(s.Config, apiGroup, s.Middleware, s.ModelHandler, s.RepoCommonHandler)

	// Dataset routes
	createDatasetRoutes(s.Config, apiGroup, s.DsHandler, s.RepoCommonHandler, s.Middleware)

	// Code routes
	createCodeRoutes(s.Config, apiGroup, s.CodeHandler, s.RepoCommonHandler, s.Middleware)

	// space routers
	createSpaceRoutes(s.Config, apiGroup, s.SpaceHandler, s.RepoCommonHandler, s.Middleware)

	spaceResource := apiGroup.Group("space_resources")
	{
		spaceResource.GET("", s.SpaceResourceHandler.Index)
		spaceResource.POST("", s.Middleware.NeedAdmin(), s.SpaceResourceHandler.Create)
		spaceResource.PUT("/:id", s.Middleware.NeedAdmin(), s.SpaceResourceHandler.Update)
		spaceResource.DELETE("/:id", s.Middleware.NeedAdmin(), s.SpaceResourceHandler.Delete)
	}

	spaceSdk := apiGroup.Group("space_sdks")
	{
		spaceSdk.GET("", s.SpaceSdkHandler.Index)
		spaceSdk.POST("", s.Middleware.NeedAPIKey(), s.SpaceSdkHandler.Create)
		spaceSdk.PUT("/:id", s.Middleware.NeedAPIKey(), s.SpaceSdkHandler.Update)
		spaceSdk.DELETE("/:id", s.Middleware.NeedAPIKey(), s.SpaceSdkHandler.Delete)
	}

	createUserRoutes(apiGroup, s.Middleware, s.UserProxyHandler, s.UserHandler)
	tokenGroup := apiGroup.Group("token")
	{
		tokenGroup.POST("/:app/:token_name", s.UserProxyHandler.ProxyToApi("/api/v1/token/%s/%s", "app", "token_name"))
		tokenGroup.PUT("/:app/:token_name", s.UserProxyHandler.ProxyToApi("/api/v1/token/%s/%s", "app", "token_name"))
		tokenGroup.DELETE("/:app/:token_name", s.UserProxyHandler.ProxyToApi("/api/v1/token/%s/%s", "app", "token_name"))
		// check token info
		tokenGroup.GET("/:token_value", s.Middleware.NeedAPIKey(), s.UserProxyHandler.ProxyToApi("/api/v1/token/%s", "token_value"))
	}

	{
		apiGroup.GET("/user/:username/ssh_keys", s.SshKeyHandler.Index)
		apiGroup.POST("/user/:username/ssh_keys", s.SshKeyHandler.Create)
		apiGroup.DELETE("/user/:username/ssh_key/:name", s.SshKeyHandler.Delete)
	}

	{
		apiGroup.GET("/organizations", s.UserProxyHandler.Proxy)
		apiGroup.POST("/organizations", s.UserProxyHandler.Proxy)
		apiGroup.GET("/organization/:namespace", s.UserProxyHandler.ProxyToApi("/api/v1/organization/%s", "namespace"))
		apiGroup.PUT("/organization/:namespace", s.UserProxyHandler.ProxyToApi("/api/v1/organization/%s", "namespace"))
		apiGroup.DELETE("/organization/:namespace", s.UserProxyHandler.ProxyToApi("/api/v1/organization/%s", "namespace"))
		// Organization assets
		apiGroup.GET("/organization/:namespace/models", s.OrgHandler.Models)
		apiGroup.GET("/organization/:namespace/datasets", s.OrgHandler.Datasets)
		apiGroup.GET("/organization/:namespace/codes", s.OrgHandler.Codes)
		apiGroup.GET("/organization/:namespace/spaces", s.OrgHandler.Spaces)
		apiGroup.GET("/organization/:namespace/collections", s.OrgHandler.Collections)
		apiGroup.GET("/organization/:namespace/prompts", s.OrgHandler.Prompts)
	}

	{
		apiGroup.GET("/organization/:namespace/members", s.UserProxyHandler.ProxyToApi("/api/v1/organization/%s/members", "namespace"))
		apiGroup.POST("/organization/:namespace/members", s.UserProxyHandler.ProxyToApi("/api/v1/organization/%s/members", "namespace"))
		apiGroup.GET("/organization/:namespace/members/:username", s.UserProxyHandler.ProxyToApi("/api/v1/organization/%s/members/%s", "namespace", "username"))
		apiGroup.PUT("/organization/:namespace/members/:username", s.UserProxyHandler.ProxyToApi("/api/v1/organization/%s/members/%s", "namespace", "username"))
		apiGroup.DELETE("/organization/:namespace/members/:username", s.UserProxyHandler.ProxyToApi("/api/v1/organization/%s/members/%s", "namespace", "username"))
	}

	// Tag
	createTagsRoutes(apiGroup, s.TagCtrl)

	// JWT token
	apiGroup.POST("/jwt/token", s.Middleware.NeedAPIKey(), s.UserProxyHandler.Proxy)
	apiGroup.GET("/jwt/:token", s.Middleware.NeedAPIKey(), s.UserProxyHandler.ProxyToApi("/api/v1/jwt/%s", "token"))
	apiGroup.GET("/users", s.UserProxyHandler.Proxy)

	// callback
	apiGroup.POST("/callback/git", s.CallbackCtrl.Handle)
	apiGroup.GET("/callback/casdoor", s.UserProxyHandler.Proxy)
	// Sensive check
	if s.Config.SensitiveCheck.Enable {
		sensitiveCtrl, err := handler.NewSensitiveHandler(s.Config)
		if err != nil {
			return fmt.Errorf("error creating sensitive handler:%w", err)
		}
		apiGroup.POST("/sensitive/text", sensitiveCtrl.Text)
		apiGroup.POST("/sensitive/image", sensitiveCtrl.Image)
	}

	// MirrorSource
	apiGroup.GET("/mirrors", s.MirrorHandler.Index)

	mirror := apiGroup.Group("/mirror")
	{
		mirror.GET("/sources", s.MsHandler.Index)
		mirror.POST("/sources", s.MsHandler.Create)
		mirror.PUT("/sources/:id", s.MsHandler.Update)
		mirror.DELETE("/sources/:id", s.MsHandler.Delete)
		mirror.GET("/sources/:id", s.MsHandler.Get)
		mirror.POST("/repo", s.MirrorHandler.CreateMirrorRepo)
		mirror.GET("/repos", s.MirrorHandler.Repos)

	}

	collections := apiGroup.Group("/collections")
	{
		// list all collection
		collections.GET("", s.CollectionHandler.Index)
		collections.POST("", s.CollectionHandler.Create)
		collections.GET("/:id", s.CollectionHandler.GetCollection)
		collections.PUT("/:id", s.CollectionHandler.UpdateCollection)
		collections.DELETE("/:id", s.CollectionHandler.DeleteCollection)
		collections.POST("/:id/repos", s.CollectionHandler.AddRepoToCollection)
		collections.DELETE("/:id/repos", s.CollectionHandler.RemoveRepoFromCollection)
	}

	// cluster infos
	cluster := apiGroup.Group("/cluster")
	{
		cluster.GET("", s.ClusterHandler.Index)
		cluster.GET("/:id", s.ClusterHandler.GetClusterById)
		cluster.PUT("/:id", s.Middleware.NeedAPIKey(), s.ClusterHandler.Update)
	}

	event := apiGroup.Group("/events")
	event.POST("", s.EventHandler.Create)

	// routes for broadcast
	broadcast := apiGroup.Group("/broadcasts")
	adminBroadcast := apiGroup.Group("/admin/broadcasts")
	adminBroadcast.Use(s.Middleware.NeedAdmin())

	broadcast.GET("/active", s.BroadcastHandler.Active)
	adminBroadcast.POST("", s.BroadcastHandler.Create)
	adminBroadcast.PUT("/:id", s.BroadcastHandler.Update)
	adminBroadcast.GET("", s.BroadcastHandler.Index)
	adminBroadcast.GET("/:id", s.BroadcastHandler.Show)
	broadcast.GET("/:id", s.BroadcastHandler.Show)
	// end routes for broadcast
	createRuntimeFrameworkRoutes(
		apiGroup, s.Middleware, s.ModelHandler, s.RuntimeArchHandler, s.RepoCommonHandler,
	)

	syncGroup := apiGroup.Group("sync")
	{
		syncGroup.GET("/version/latest", s.SyncHandler.Latest)
		// syncGroup.GET("/version/oldest", syncHandler.Oldest)
		syncGroup.GET("/client_setting", s.SyncClientSettingHandler.Show)
		syncGroup.POST("/client_setting", s.SyncClientSettingHandler.Create)
	}

	recomGroup := apiGroup.Group("/recom")
	{
		recomGroup.POST("opweight", s.Middleware.NeedAdmin(), s.RecomHandler.SetOpWeight)
	}

	// telemetry
	teleGroup := apiGroup.Group("/telemetry")
	teleGroup.POST("/usage", s.TelemetryHandler.Usage)

	// internal API for gitaly to check request permissions
	needGitlabShellJWTToken := s.Middleware.CheckGitlabShellJWTToken()
	r.GET("/api/v4/internal/authorized_keys", needGitlabShellJWTToken, s.InternalHandler.GetAuthorizedKeys)
	r.POST("/api/v4/internal/allowed", needGitlabShellJWTToken, s.InternalHandler.SSHAllowed)
	r.POST("/api/v4/internal/pre_receive", needGitlabShellJWTToken, s.InternalHandler.PreReceive)
	r.POST("api/v4/internal/lfs_authenticate", needGitlabShellJWTToken, s.InternalHandler.LfsAuthenticate)
	r.POST("/api/v4/internal/post_receive", needGitlabShellJWTToken, s.InternalHandler.PostReceive)
	internalGroup := apiGroup.Group("/internal")
	{
		internalGroup.POST("/allowed", needGitlabShellJWTToken, s.InternalHandler.Allowed)
		internalGroup.POST("/pre_receive", needGitlabShellJWTToken, s.InternalHandler.PreReceive)
		internalGroup.POST("/post_receive", needGitlabShellJWTToken, s.InternalHandler.PostReceive)
	}
	createDiscussionRoutes(apiGroup, s.Middleware.NeedAPIKey(), s.DiscussionHandler)

	// prompt
	createPromptRoutes(apiGroup, s.PromptHandler)
	// Dataset viewer proxy
	createDataViewerRoutes(apiGroup, s.DsViewerHandler)

	// space template
	templateHandler, err := handler.NewSpaceTemplateHandler(s.Config)
	if err != nil {
		return fmt.Errorf("error creating space template proxy:%w", err)
	}
	createSpaceTemplateRoutes(apiGroup, s.Middleware, templateHandler)

	return nil
}

func createEvaluationRoutes(apiGroup *gin.RouterGroup, evaluationHandler *handler.EvaluationHandler) {
	// Models routes
	evaluationsGroup := apiGroup.Group("/evaluations")
	{
		evaluationsGroup.POST("", evaluationHandler.RunEvaluation)
		evaluationsGroup.DELETE("/:id", evaluationHandler.DeleteEvaluation)
		evaluationsGroup.GET("/:id", evaluationHandler.GetEvaluation)
	}
}

func createModelRoutes(config *config.Config, apiGroup *gin.RouterGroup, middleware *middleware.Middleware, modelHandler *handler.ModelHandler, repoCommonHandler *handler.RepoHandler) {
	// Models routes
	modelsGroup := apiGroup.Group("/models")
	{
		modelsGroup.POST("", modelHandler.Create)
		modelsGroup.GET("", modelHandler.Index)
		modelsGroup.PUT("/:namespace/:name", modelHandler.Update)
		modelsGroup.DELETE("/:namespace/:name", modelHandler.Delete)
		modelsGroup.GET("/:namespace/:name", modelHandler.Show)
		modelsGroup.GET("/:namespace/:name/all_files", modelHandler.AllFiles)
		modelsGroup.GET("/:namespace/:name/relations", modelHandler.Relations)
		modelsGroup.PUT("/:namespace/:name/relations", modelHandler.SetRelations)
		modelsGroup.POST("/:namespace/:name/relations/dataset", modelHandler.AddDatasetRelation)
		modelsGroup.DELETE("/:namespace/:name/relations/dataset", modelHandler.DelDatasetRelation)
		modelsGroup.GET("/:namespace/:name/branches", middleware.RepoType(types.ModelRepo), repoCommonHandler.Branches)
		modelsGroup.GET("/:namespace/:name/tags", middleware.RepoType(types.ModelRepo), repoCommonHandler.Tags)
		// update tags of a certain category
		modelsGroup.POST("/:namespace/:name/tags/:category", middleware.RepoType(types.ModelRepo), repoCommonHandler.UpdateTags)
		modelsGroup.GET("/:namespace/:name/last_commit", middleware.RepoType(types.ModelRepo), repoCommonHandler.LastCommit)
		modelsGroup.GET("/:namespace/:name/commit/:commit_id", middleware.RepoType(types.ModelRepo), repoCommonHandler.CommitWithDiff)
		modelsGroup.GET("/:namespace/:name/tree", middleware.RepoType(types.ModelRepo), repoCommonHandler.Tree)
		modelsGroup.GET("/:namespace/:name/refs/:ref/tree/*path", middleware.RepoType(types.ModelRepo), repoCommonHandler.TreeV2)
		modelsGroup.GET("/:namespace/:name/refs/:ref/logs_tree/*path", middleware.RepoType(types.ModelRepo), repoCommonHandler.LogsTree)
		modelsGroup.GET("/:namespace/:name/commits", middleware.RepoType(types.ModelRepo), repoCommonHandler.Commits)
		modelsGroup.GET("/:namespace/:name/raw/*file_path", middleware.RepoType(types.ModelRepo), repoCommonHandler.FileRaw)
		modelsGroup.GET("/:namespace/:name/blob/*file_path", middleware.RepoType(types.ModelRepo), repoCommonHandler.FileInfo)
		// The DownloadFile method differs from the SDKDownload interface in a few ways

		// 1.When passing the file_path parameter to the SDKDownload method,
		// it only needs to pass the path of the file itself,
		// whether it is an lfs file or a non-lfs file.
		// The DownloadFile has a different file_path format for lfs files and non-lfs files,
		// and an lfs parameter needs to be added.
		// 2. DownloadFile returns an object store url for lfs files, while SDKDownload redirects directly.
		modelsGroup.GET("/:namespace/:name/download/*file_path", middleware.RepoType(types.ModelRepo), repoCommonHandler.DownloadFile)
		modelsGroup.GET("/:namespace/:name/resolve/*file_path", middleware.RepoType(types.ModelRepo), repoCommonHandler.ResolveDownload)
		modelsGroup.POST("/:namespace/:name/raw/*file_path", middleware.RepoType(types.ModelRepo), repoCommonHandler.CreateFile)
		modelsGroup.PUT("/:namespace/:name/raw/*file_path", middleware.RepoType(types.ModelRepo), repoCommonHandler.UpdateFile)
		modelsGroup.POST("/:namespace/:name/update_downloads", middleware.RepoType(types.ModelRepo), repoCommonHandler.UpdateDownloads)
		modelsGroup.PUT("/:namespace/:name/incr_downloads", middleware.RepoType(types.ModelRepo), repoCommonHandler.IncrDownloads)
		modelsGroup.POST("/:namespace/:name/upload_file", middleware.RepoType(types.ModelRepo), repoCommonHandler.UploadFile)
		modelsGroup.POST("/:namespace/:name/mirror", middleware.RepoType(types.ModelRepo), repoCommonHandler.CreateMirror)
		modelsGroup.GET("/:namespace/:name/mirror", middleware.RepoType(types.ModelRepo), repoCommonHandler.GetMirror)
		modelsGroup.PUT("/:namespace/:name/mirror", middleware.RepoType(types.ModelRepo), repoCommonHandler.UpdateMirror)
		modelsGroup.DELETE("/:namespace/:name/mirror", middleware.RepoType(types.ModelRepo), repoCommonHandler.DeleteMirror)
		modelsGroup.POST("/:namespace/:name/mirror/sync", middleware.RepoType(types.ModelRepo), repoCommonHandler.SyncMirror)

		// mirror from SaaS, only on-premises available
		if !config.Saas {
			modelsGroup.POST("/:namespace/:name/mirror_from_saas", middleware.RepoType(types.ModelRepo), repoCommonHandler.MirrorFromSaas)
		}

		// runtime framework
		modelsGroup.GET("/:namespace/:name/runtime_framework", middleware.RepoType(types.ModelRepo), repoCommonHandler.RuntimeFrameworkList)
		// list model inference
		modelsGroup.GET("/:namespace/:name/run", middleware.RepoType(types.ModelRepo), repoCommonHandler.DeployList)
		// deploy model as inference
		modelsGroup.POST("/:namespace/:name/run", middleware.RepoType(types.ModelRepo), modelHandler.DeployDedicated)
		// delete a deployed inference
		modelsGroup.DELETE("/:namespace/:name/run/:id", middleware.RepoType(types.ModelRepo), modelHandler.DeployDelete)
		modelsGroup.GET("/:namespace/:name/run/:id", middleware.RepoType(types.ModelRepo), repoCommonHandler.DeployDetail)
		modelsGroup.GET("/:namespace/:name/run/:id/status", middleware.RepoType(types.ModelRepo), repoCommonHandler.DeployStatus)
		modelsGroup.GET("/:namespace/:name/run/:id/logs/:instance", middleware.RepoType(types.ModelRepo), repoCommonHandler.DeployInstanceLogs)
		modelsGroup.PUT("/:namespace/:name/run/:id", middleware.RepoType(types.ModelRepo), repoCommonHandler.DeployUpdate)
		modelsGroup.PUT("/:namespace/:name/run/:id/stop", middleware.RepoType(types.ModelRepo), modelHandler.DeployStop)
		modelsGroup.PUT("/:namespace/:name/run/:id/start", middleware.RepoType(types.ModelRepo), modelHandler.DeployStart)

		// runtime framework for both finetune and inference
		modelsGroup.GET("/runtime_framework", middleware.RepoType(types.ModelRepo), repoCommonHandler.RuntimeFrameworkListWithType)

		// deploy model as finetune instance
		modelsGroup.POST("/:namespace/:name/finetune", middleware.RepoType(types.ModelRepo), modelHandler.FinetuneCreate)
		// stop a finetune instance
		modelsGroup.PUT("/:namespace/:name/finetune/:id/stop", middleware.RepoType(types.ModelRepo), modelHandler.FinetuneStop)
		// start a finetune instance
		modelsGroup.PUT("/:namespace/:name/finetune/:id/start", middleware.RepoType(types.ModelRepo), modelHandler.FinetuneStart)
		// delete a finetune instance
		modelsGroup.DELETE("/:namespace/:name/finetune/:id", middleware.RepoType(types.ModelRepo), modelHandler.FinetuneDelete)

		// deploy model as serverless
		modelsGroup.GET("/:namespace/:name/serverless", middleware.RepoType(types.ModelRepo), modelHandler.GetDeployServerless)
		modelsGroup.POST("/:namespace/:name/serverless", middleware.RepoType(types.ModelRepo), modelHandler.DeployServerless)
		modelsGroup.PUT("/:namespace/:name/serverless/:id/start", middleware.RepoType(types.ModelRepo), modelHandler.ServerlessStart)
		modelsGroup.PUT("/:namespace/:name/serverless/:id/stop", middleware.RepoType(types.ModelRepo), modelHandler.ServerlessStop)
		modelsGroup.GET("/:namespace/:name/serverless/:id", middleware.RepoType(types.ModelRepo), repoCommonHandler.ServerlessDetail)
		modelsGroup.GET("/:namespace/:name/serverless/:id/status", middleware.RepoType(types.ModelRepo), repoCommonHandler.ServerlessStatus)
		modelsGroup.GET("/:namespace/:name/serverless/:id/logs/:instance", middleware.RepoType(types.ModelRepo), repoCommonHandler.ServerlessLogs)
		modelsGroup.PUT("/:namespace/:name/serverless/:id", middleware.RepoType(types.ModelRepo), repoCommonHandler.ServerlessUpdate)
	}
}

func createDatasetRoutes(config *config.Config, apiGroup *gin.RouterGroup, dsHandler *handler.DatasetHandler, repoCommonHandler *handler.RepoHandler, middleware *middleware.Middleware) {
	datasetsGroup := apiGroup.Group("/datasets")
	{
		datasetsGroup.POST("", dsHandler.Create)
		datasetsGroup.GET("", dsHandler.Index)
		datasetsGroup.PUT("/:namespace/:name", dsHandler.Update)
		datasetsGroup.DELETE("/:namespace/:name", dsHandler.Delete)
		datasetsGroup.GET("/:namespace/:name", dsHandler.Show)
		datasetsGroup.GET("/:namespace/:name/all_files", dsHandler.AllFiles)
		datasetsGroup.GET("/:namespace/:name/relations", dsHandler.Relations)
		datasetsGroup.GET("/:namespace/:name/branches", middleware.RepoType(types.DatasetRepo), repoCommonHandler.Branches)
		datasetsGroup.GET("/:namespace/:name/tags", middleware.RepoType(types.DatasetRepo), repoCommonHandler.Tags)
		// update tags of a certain category
		datasetsGroup.POST("/:namespace/:name/tags/:category", middleware.RepoType(types.DatasetRepo), repoCommonHandler.UpdateTags)
		datasetsGroup.GET("/:namespace/:name/last_commit", middleware.RepoType(types.DatasetRepo), repoCommonHandler.LastCommit)
		datasetsGroup.GET("/:namespace/:name/commit/:commit_id", middleware.RepoType(types.DatasetRepo), repoCommonHandler.CommitWithDiff)
		datasetsGroup.GET("/:namespace/:name/tree", middleware.RepoType(types.DatasetRepo), repoCommonHandler.Tree)
		datasetsGroup.GET("/:namespace/:name/refs/:ref/tree/*path", middleware.RepoType(types.DatasetRepo), repoCommonHandler.TreeV2)
		datasetsGroup.GET("/:namespace/:name/refs/:ref/logs_tree/*path", middleware.RepoType(types.DatasetRepo), repoCommonHandler.LogsTree)
		datasetsGroup.GET("/:namespace/:name/commits", middleware.RepoType(types.DatasetRepo), repoCommonHandler.Commits)
		datasetsGroup.POST("/:namespace/:name/raw/*file_path", middleware.RepoType(types.DatasetRepo), repoCommonHandler.CreateFile)
		datasetsGroup.GET("/:namespace/:name/raw/*file_path", middleware.RepoType(types.DatasetRepo), repoCommonHandler.FileRaw)
		datasetsGroup.GET("/:namespace/:name/blob/*file_path", middleware.RepoType(types.DatasetRepo), repoCommonHandler.FileInfo)
		datasetsGroup.GET("/:namespace/:name/download/*file_path", middleware.RepoType(types.DatasetRepo), repoCommonHandler.DownloadFile)
		datasetsGroup.GET("/:namespace/:name/resolve/*file_path", middleware.RepoType(types.DatasetRepo), repoCommonHandler.ResolveDownload)
		datasetsGroup.PUT("/:namespace/:name/raw/*file_path", middleware.RepoType(types.DatasetRepo), repoCommonHandler.UpdateFile)
		datasetsGroup.POST("/:namespace/:name/update_downloads", middleware.RepoType(types.DatasetRepo), repoCommonHandler.UpdateDownloads)
		datasetsGroup.PUT("/:namespace/:name/incr_downloads", middleware.RepoType(types.DatasetRepo), repoCommonHandler.IncrDownloads)
		datasetsGroup.POST("/:namespace/:name/upload_file", middleware.RepoType(types.DatasetRepo), repoCommonHandler.UploadFile)
		datasetsGroup.POST("/:namespace/:name/mirror", middleware.RepoType(types.DatasetRepo), repoCommonHandler.CreateMirror)
		datasetsGroup.GET("/:namespace/:name/mirror", middleware.RepoType(types.DatasetRepo), repoCommonHandler.GetMirror)
		datasetsGroup.PUT("/:namespace/:name/mirror", middleware.RepoType(types.DatasetRepo), repoCommonHandler.UpdateMirror)
		datasetsGroup.DELETE("/:namespace/:name/mirror", middleware.RepoType(types.DatasetRepo), repoCommonHandler.DeleteMirror)
		datasetsGroup.POST("/:namespace/:name/mirror/sync", middleware.RepoType(types.DatasetRepo), repoCommonHandler.SyncMirror)

		// mirror from SaaS, only on-premises available
		if !config.Saas {
			datasetsGroup.POST("/:namespace/:name/mirror_from_saas", middleware.RepoType(types.DatasetRepo), repoCommonHandler.MirrorFromSaas)
		}
	}
}

func createCodeRoutes(config *config.Config, apiGroup *gin.RouterGroup, codeHandler *handler.CodeHandler, repoCommonHandler *handler.RepoHandler, middleware *middleware.Middleware) {
	codesGroup := apiGroup.Group("/codes")
	{
		codesGroup.POST("", codeHandler.Create)
		codesGroup.GET("", codeHandler.Index)
		codesGroup.PUT("/:namespace/:name", codeHandler.Update)
		codesGroup.DELETE("/:namespace/:name", codeHandler.Delete)
		codesGroup.GET("/:namespace/:name", codeHandler.Show)
		codesGroup.GET("/:namespace/:name/relations", codeHandler.Relations)
		codesGroup.GET("/:namespace/:name/branches", middleware.RepoType(types.CodeRepo), repoCommonHandler.Branches)
		codesGroup.GET("/:namespace/:name/tags", middleware.RepoType(types.CodeRepo), repoCommonHandler.Tags)
		// update tags of a certain category
		codesGroup.POST("/:namespace/:name/tags/:category", middleware.RepoType(types.CodeRepo), repoCommonHandler.UpdateTags)
		codesGroup.GET("/:namespace/:name/last_commit", middleware.RepoType(types.CodeRepo), repoCommonHandler.LastCommit)
		codesGroup.GET("/:namespace/:name/commit/:commit_id", middleware.RepoType(types.CodeRepo), repoCommonHandler.CommitWithDiff)
		codesGroup.GET("/:namespace/:name/tree", middleware.RepoType(types.CodeRepo), repoCommonHandler.Tree)
		codesGroup.GET("/:namespace/:name/refs/:ref/tree/*path", middleware.RepoType(types.CodeRepo), repoCommonHandler.TreeV2)
		codesGroup.GET("/:namespace/:name/refs/:ref/logs_tree/*path", middleware.RepoType(types.CodeRepo), repoCommonHandler.LogsTree)
		codesGroup.GET("/:namespace/:name/commits", middleware.RepoType(types.CodeRepo), repoCommonHandler.Commits)
		codesGroup.POST("/:namespace/:name/raw/*file_path", middleware.RepoType(types.CodeRepo), repoCommonHandler.CreateFile)
		codesGroup.GET("/:namespace/:name/raw/*file_path", middleware.RepoType(types.CodeRepo), repoCommonHandler.FileRaw)
		codesGroup.GET("/:namespace/:name/blob/*file_path", middleware.RepoType(types.CodeRepo), repoCommonHandler.FileInfo)
		codesGroup.GET("/:namespace/:name/download/*file_path", middleware.RepoType(types.CodeRepo), repoCommonHandler.DownloadFile)
		codesGroup.GET("/:namespace/:name/resolve/*file_path", middleware.RepoType(types.CodeRepo), repoCommonHandler.ResolveDownload)
		codesGroup.PUT("/:namespace/:name/raw/*file_path", middleware.RepoType(types.CodeRepo), repoCommonHandler.UpdateFile)
		codesGroup.POST("/:namespace/:name/update_downloads", middleware.RepoType(types.CodeRepo), repoCommonHandler.UpdateDownloads)
		codesGroup.PUT("/:namespace/:name/incr_downloads", middleware.RepoType(types.CodeRepo), repoCommonHandler.IncrDownloads)
		codesGroup.POST("/:namespace/:name/upload_file", middleware.RepoType(types.CodeRepo), repoCommonHandler.UploadFile)
		codesGroup.POST("/:namespace/:name/mirror", middleware.RepoType(types.CodeRepo), repoCommonHandler.CreateMirror)
		codesGroup.GET("/:namespace/:name/mirror", middleware.RepoType(types.CodeRepo), repoCommonHandler.GetMirror)
		codesGroup.PUT("/:namespace/:name/mirror", middleware.RepoType(types.CodeRepo), repoCommonHandler.UpdateMirror)
		codesGroup.DELETE("/:namespace/:name/mirror", middleware.RepoType(types.CodeRepo), repoCommonHandler.DeleteMirror)
		codesGroup.POST("/:namespace/:name/mirror/sync", middleware.RepoType(types.CodeRepo), repoCommonHandler.SyncMirror)

		// mirror from SaaS, only on-premises available
		if !config.Saas {
			codesGroup.POST("/:namespace/:name/mirror_from_saas", middleware.RepoType(types.CodeRepo), repoCommonHandler.MirrorFromSaas)
		}
	}
}

func createSpaceRoutes(config *config.Config, apiGroup *gin.RouterGroup, spaceHandler *handler.SpaceHandler, repoCommonHandler *handler.RepoHandler, middleware *middleware.Middleware) {
	spaces := apiGroup.Group("/spaces")
	{
		// list all spaces
		spaces.GET("", spaceHandler.Index)
		spaces.POST("", spaceHandler.Create)
		// show a user or org's space
		spaces.GET("/:namespace/:name", spaceHandler.Show)
		spaces.PUT("/:namespace/:name", spaceHandler.Update)
		spaces.DELETE("/:namespace/:name", spaceHandler.Delete)
		// depoly and start running the space
		spaces.POST("/:namespace/:name/run", spaceHandler.Run)
		// wake a sleeping space
		spaces.POST("/:namespace/:name/wakeup", spaceHandler.Wakeup)
		// stop running space
		spaces.POST("/:namespace/:name/stop", spaceHandler.Stop)
		// pull space running status
		spaces.GET("/:namespace/:name/status", spaceHandler.Status)
		// pull space building and running logs
		spaces.GET("/:namespace/:name/logs", spaceHandler.Logs)
		// call space webhook api
		spaces.POST("/:namespace/:name/webhook", nil)

		spaces.GET("/:namespace/:name/branches", middleware.RepoType(types.SpaceRepo), repoCommonHandler.Branches)
		spaces.GET("/:namespace/:name/tags", middleware.RepoType(types.SpaceRepo), repoCommonHandler.Tags)
		// update tags of a certain category
		spaces.POST("/:namespace/:name/tags/:category", middleware.RepoType(types.SpaceRepo), repoCommonHandler.UpdateTags)
		spaces.GET("/:namespace/:name/last_commit", middleware.RepoType(types.SpaceRepo), repoCommonHandler.LastCommit)
		spaces.GET("/:namespace/:name/commit/:commit_id", middleware.RepoType(types.SpaceRepo), repoCommonHandler.CommitWithDiff)
		spaces.GET("/:namespace/:name/tree", middleware.RepoType(types.SpaceRepo), repoCommonHandler.Tree)
		spaces.GET("/:namespace/:name/refs/:ref/tree/*path", middleware.RepoType(types.SpaceRepo), repoCommonHandler.TreeV2)
		spaces.GET("/:namespace/:name/refs/:ref/logs_tree/*path", middleware.RepoType(types.SpaceRepo), repoCommonHandler.LogsTree)
		spaces.GET("/:namespace/:name/commits", middleware.RepoType(types.SpaceRepo), repoCommonHandler.Commits)
		spaces.POST("/:namespace/:name/raw/*file_path", middleware.RepoType(types.SpaceRepo), repoCommonHandler.CreateFile)
		spaces.GET("/:namespace/:name/raw/*file_path", middleware.RepoType(types.SpaceRepo), repoCommonHandler.FileRaw)
		spaces.GET("/:namespace/:name/blob/*file_path", middleware.RepoType(types.SpaceRepo), repoCommonHandler.FileInfo)
		spaces.GET("/:namespace/:name/download/*file_path", middleware.RepoType(types.SpaceRepo), repoCommonHandler.DownloadFile)
		spaces.GET("/:namespace/:name/resolve/*file_path", middleware.RepoType(types.SpaceRepo), repoCommonHandler.ResolveDownload)
		spaces.PUT("/:namespace/:name/raw/*file_path", middleware.RepoType(types.SpaceRepo), repoCommonHandler.UpdateFile)
		spaces.POST("/:namespace/:name/update_downloads", middleware.RepoType(types.SpaceRepo), repoCommonHandler.UpdateDownloads)
		spaces.PUT("/:namespace/:name/incr_downloads", middleware.RepoType(types.SpaceRepo), repoCommonHandler.IncrDownloads)
		spaces.POST("/:namespace/:name/upload_file", middleware.RepoType(types.SpaceRepo), repoCommonHandler.UploadFile)
		spaces.POST("/:namespace/:name/mirror", middleware.RepoType(types.SpaceRepo), repoCommonHandler.CreateMirror)
		spaces.GET("/:namespace/:name/mirror", middleware.RepoType(types.SpaceRepo), repoCommonHandler.GetMirror)
		spaces.PUT("/:namespace/:name/mirror", middleware.RepoType(types.SpaceRepo), repoCommonHandler.UpdateMirror)
		spaces.DELETE("/:namespace/:name/mirror", middleware.RepoType(types.SpaceRepo), repoCommonHandler.DeleteMirror)
		spaces.POST("/:namespace/:name/mirror/sync", middleware.RepoType(types.SpaceRepo), repoCommonHandler.SyncMirror)

		// mirror from SaaS, only on-premises available
		if !config.Saas {
			spaces.POST("/:namespace/:name/mirror_from_saas", middleware.RepoType(types.SpaceRepo), repoCommonHandler.MirrorFromSaas)
		}
		spaces.GET("/:namespace/:name/run", middleware.RepoType(types.SpaceRepo), repoCommonHandler.DeployList)
		spaces.GET("/:namespace/:name/run/:id", middleware.RepoType(types.SpaceRepo), repoCommonHandler.DeployDetail)
		spaces.GET("/:namespace/:name/run/:id/status", middleware.RepoType(types.SpaceRepo), repoCommonHandler.DeployStatus)
		spaces.GET("/:namespace/:name/run/:id/logs/:instance", middleware.RepoType(types.SpaceRepo), repoCommonHandler.DeployInstanceLogs)
	}
}

func createUserRoutes(apiGroup *gin.RouterGroup, middleware *middleware.Middleware, userProxyHandler *handler.InternalServiceProxyHandler, userHandler *handler.UserHandler) {
	// depricated
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
		// User likes
		apiGroup.PUT("/user/:username/likes/:repo_id", userHandler.LikesAdd)
		apiGroup.DELETE("/user/:username/likes/:repo_id", userHandler.LikesDelete)
		apiGroup.GET("/user/:username/likes/spaces", userHandler.LikesSpaces)
		apiGroup.GET("/user/:username/likes/codes", userHandler.LikesCodes)
		apiGroup.GET("/user/:username/likes/models", userHandler.LikesModels)
		apiGroup.GET("/user/:username/likes/datasets", userHandler.LikesDatasets)
		apiGroup.GET("/user/:username/run/:repo_type", userHandler.GetRunDeploys)
		apiGroup.GET("/user/:username/finetune/instances", userHandler.GetFinetuneInstances)
		// User evaluations
		apiGroup.GET("/user/:username/evaluations", userHandler.GetEvaluations)
	}

	// User collection
	apiGroup.GET("/user/:username/collections", userHandler.UserCollections)
	apiGroup.GET("/user/:username/likes/collections", userHandler.LikesCollections)
	apiGroup.PUT("/user/:username/likes/collections/:id", userHandler.LikeCollection)
	apiGroup.DELETE("/user/:username/likes/collections/:id", userHandler.UnLikeCollection)
	// user owned tokens
	apiGroup.GET("/user/:username/tokens", userProxyHandler.ProxyToApi("/api/v1/user/%s/tokens", "username"))

	// serverless list
	apiGroup.GET("/user/:username/run/serverless", middleware.NeedAPIKey(), userHandler.GetRunServerless)
}

func createRuntimeFrameworkRoutes(apiGroup *gin.RouterGroup, middleware *middleware.Middleware, modelHandler *handler.ModelHandler, runtimeArchHandler *handler.RuntimeArchitectureHandler, repoCommonHandler *handler.RepoHandler) {
	needAPIKey := middleware.NeedAPIKey()
	runtimeFramework := apiGroup.Group("/runtime_framework")
	{
		runtimeFramework.GET("/:id/models", modelHandler.ListByRuntimeFrameworkID)
		runtimeFramework.GET("", modelHandler.ListAllRuntimeFramework)
		runtimeFramework.POST("", repoCommonHandler.RuntimeFrameworkCreate)
		runtimeFramework.PUT("/:id", repoCommonHandler.RuntimeFrameworkUpdate)
		runtimeFramework.DELETE("/:id", repoCommonHandler.RuntimeFrameworkDelete)
		runtimeFramework.PUT("/:id/models", modelHandler.UpdateModelRuntimeFrameworks)
		runtimeFramework.DELETE("/:id/models", modelHandler.DeleteModelRuntimeFrameworks)
		runtimeFramework.GET("/models", modelHandler.ListModelsOfRuntimeFrameworks)

		runtimeFramework.GET("/:id/architecture", needAPIKey, runtimeArchHandler.ListByRuntimeFrameworkID)
		runtimeFramework.PUT("/:id/architecture", needAPIKey, runtimeArchHandler.UpdateArchitecture)
		runtimeFramework.DELETE("/:id/architecture", needAPIKey, runtimeArchHandler.DeleteArchitecture)
		runtimeFramework.POST("/:id/scan", needAPIKey, runtimeArchHandler.ScanArchitecture)
	}
}

func createMappingRoutes(r *gin.Engine, group string, hfdsHandler *handler.HFDatasetHandler, repoCommonHandler *handler.RepoHandler, modelHandler *handler.ModelHandler, userHandler *handler.UserHandler, middleware *middleware.Middleware) {
	// Huggingface SDK routes
	hfGroup := r.Group(group)
	{
		hfGroup.GET("/:namespace/:name/resolve/:branch/*file_path", middleware.RepoMapping(types.ModelRepo), repoCommonHandler.SDKDownload)
		hfGroup.HEAD("/:namespace/:name/resolve/:branch/*file_path", middleware.RepoMapping(types.ModelRepo), repoCommonHandler.HeadSDKDownload)
		hfdsFileGroup := hfGroup.Group("/datasets")
		{
			hfdsFileGroup.GET("/:namespace/:name/resolve/:branch/*file_path", middleware.RepoMapping(types.DatasetRepo), repoCommonHandler.SDKDownload)
			hfdsFileGroup.HEAD("/:namespace/:name/resolve/:branch/*file_path", middleware.RepoMapping(types.DatasetRepo), repoCommonHandler.HeadSDKDownload)
		}
		hfAPIGroup := hfGroup.Group("/api")
		{
			hfAPIGroup.GET("/whoami-v2", userHandler.UserPermission)
			hfModelAPIGroup := hfAPIGroup.Group("/models")
			{
				// compatible with HF model info api, used for sdk like this:  huggingface_hub.model_info(repo_id, revision)
				hfModelAPIGroup.GET("/:namespace/:name/revision/:ref", middleware.RepoMapping(types.ModelRepo), modelHandler.SDKModelInfo)
				hfModelAPIGroup.GET("/:namespace/:name", middleware.RepoMapping(types.ModelRepo), modelHandler.SDKModelInfo)
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
		}
	}
}

func createDiscussionRoutes(apiGroup *gin.RouterGroup, needAPIKey gin.HandlerFunc, discussionHandler *handler.DiscussionHandler) {
	apiGroup.POST("/:repo_type/:namespace/:name/discussions", discussionHandler.CreateRepoDiscussion)
	apiGroup.GET("/:repo_type/:namespace/:name/discussions", discussionHandler.ListRepoDiscussions)
	apiGroup.GET("/discussions/:id", discussionHandler.ShowDiscussion)
	apiGroup.PUT("/discussions/:id", discussionHandler.UpdateDiscussion)
	apiGroup.DELETE("/discussions/:id", discussionHandler.DeleteDiscussion)
	apiGroup.POST("/discussions/:id/comments", discussionHandler.CreateDiscussionComment)
	apiGroup.GET("/discussions/:id/comments", discussionHandler.ListDiscussionComments)
	apiGroup.PUT("/discussions/:id/comments/:comment_id", discussionHandler.UpdateComment)
	apiGroup.DELETE("/discussions/:id/comments/:comment_id", discussionHandler.DeleteComment)
}

func createPromptRoutes(apiGroup *gin.RouterGroup, promptHandler *handler.PromptHandler) {
	promptGrp := apiGroup.Group("/prompts")
	{
		promptGrp.GET("", promptHandler.Index)
		promptGrp.GET("/:namespace/:name", promptHandler.ListPrompt)
		promptGrp.GET("/:namespace/:name/relations", promptHandler.Relations)
		promptGrp.GET("/:namespace/:name/prompt/view/*file_path", promptHandler.GetPrompt)
		promptGrp.POST("/:namespace/:name/prompt/record", promptHandler.CreatePrompt)
		promptGrp.PUT("/:namespace/:name/prompt/record/*file_path", promptHandler.UpdatePrompt)
		promptGrp.DELETE("/:namespace/:name/prompt/record/*file_path", promptHandler.DeletePrompt)

		promptGrp.PUT("/:namespace/:name/relations", promptHandler.SetRelations)
		promptGrp.POST("/:namespace/:name/relations/model", promptHandler.AddModelRelation)
		promptGrp.DELETE("/:namespace/:name/relations/model", promptHandler.DelModelRelation)

		promptGrp.POST("", promptHandler.Create)
		promptGrp.PUT("/:namespace/:name", promptHandler.Update)
		promptGrp.DELETE("/:namespace/:name", promptHandler.Delete)

		promptGrp.GET("/:namespace/:name/branches", promptHandler.Branches)
		promptGrp.GET("/:namespace/:name/tags", promptHandler.Tags)
		promptGrp.POST("/:namespace/:name/tags/:category", promptHandler.UpdateTags)
		promptGrp.POST("/:namespace/:name/update_downloads", promptHandler.UpdateDownloads)
	}
}

func createTagsRoutes(apiGroup *gin.RouterGroup, tagHandler *handler.TagsHandler) {
	tagsGrp := apiGroup.Group("/tags")
	{
		categoryGrp := tagsGrp.Group("/categories")
		{
			categoryGrp.GET("", tagHandler.AllCategories)
			categoryGrp.POST("", tagHandler.CreateCategory)
			categoryGrp.PUT("/:id", tagHandler.UpdateCategory)
			categoryGrp.DELETE("/:id", tagHandler.DeleteCategory)
		}
		tagsGrp.GET("", tagHandler.AllTags)
		tagsGrp.POST("", tagHandler.CreateTag)
		tagsGrp.GET("/:id", tagHandler.GetTagByID)
		tagsGrp.PUT("/:id", tagHandler.UpdateTag)
		tagsGrp.DELETE("/:id", tagHandler.DeleteTag)
	}
}

func createDataViewerRoutes(apiGroup *gin.RouterGroup, dsViewerHandler *handler.InternalServiceProxyHandler) {
	datasetRepoGrp := apiGroup.Group("/datasets/:namespace/:name")
	fileViewerGrp := datasetRepoGrp.Group("/viewer")
	{
		fileViewerGrp.Any("/*any", dsViewerHandler.Proxy)
	}
	dataViewerGrp := datasetRepoGrp.Group("/dataviewer")
	{
		dataViewerGrp.Any("/*any", dsViewerHandler.Proxy)
	}
}

func createSpaceTemplateRoutes(apiGroup *gin.RouterGroup, middleware *middleware.Middleware, templateHandler *handler.SpaceTemplateHandler) {
	spaceTemplateGrp := apiGroup.Group("/space_templates")
	{
		spaceTemplateGrp.GET("", middleware.NeedAdmin(), templateHandler.Index)
		spaceTemplateGrp.POST("", middleware.NeedAdmin(), templateHandler.Create)
		spaceTemplateGrp.PUT("/:id", middleware.NeedAdmin(), templateHandler.Update)
		spaceTemplateGrp.DELETE("/:id", middleware.NeedAdmin(), templateHandler.Delete)
		spaceTemplateGrp.GET("/:type", templateHandler.List)
	}
}
