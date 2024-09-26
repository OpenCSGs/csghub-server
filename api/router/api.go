package router

import (
	"fmt"
	"time"

	cache "github.com/chenyahui/gin-cache"
	"github.com/chenyahui/gin-cache/persist"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/handler/callback"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewRouter(config *config.Config, enableSwagger bool) (*gin.Engine, error) {
	r := gin.New()
	r.Use(cors.New(cors.Config{
		AllowCredentials: true,
		AllowHeaders:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowAllOrigins:  true,
	}))
	r.Use(gin.Recovery())
	r.Use(middleware.Log())
	gitHTTPHandler, err := handler.NewGitHTTPHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating git http handler:%w", err)
	}
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

	needAPIKey := middleware.OnlyAPIKeyAuthenticator(config)

	if enableSwagger {
		r.GET("/api/v1/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// User routes
	userHandler, err := handler.NewUserHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
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

	createHFRoutes(r, hfdsHandler, repoCommonHandler, modelHandler, userHandler)

	apiGroup := r.Group("/api/v1")
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

	// Model routes
	createModelRoutes(config, apiGroup, needAPIKey, modelHandler, repoCommonHandler)
	// Dataset routes
	createDatasetRoutes(config, apiGroup, dsHandler, repoCommonHandler)

	codeHandler, err := handler.NewCodeHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating code handler:%w", err)
	}
	// Code routes
	createCodeRoutes(config, apiGroup, codeHandler, repoCommonHandler)

	// Dataset viewer
	dsViewerHandler, err := handler.NewDatasetViewerHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dataset viewer handler:%w", err)
	}
	apiGroup.GET("/datasets/:namespace/:name/viewer/*file_path", dsViewerHandler.View)

	spaceHandler, err := handler.NewSpaceHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating space handler:%w", err)
	}
	// space routers
	createSpaceRoutes(config, apiGroup, spaceHandler, repoCommonHandler)

	spaceResourceHandler, err := handler.NewSpaceResourceHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating space resource handler:%w", err)
	}

	spaceResource := apiGroup.Group("space_resources")
	{
		spaceResource.GET("", spaceResourceHandler.Index)
		spaceResource.POST("", needAPIKey, spaceResourceHandler.Create)
		spaceResource.PUT("/:id", needAPIKey, spaceResourceHandler.Update)
		spaceResource.DELETE("/:id", needAPIKey, spaceResourceHandler.Delete)
	}

	spaceSdkHandler, err := handler.NewSpaceSdkHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating space sdk handler:%w", err)
	}

	spaceSdk := apiGroup.Group("space_sdks")
	{
		spaceSdk.GET("", spaceSdkHandler.Index)
		spaceSdk.POST("", needAPIKey, spaceSdkHandler.Create)
		spaceSdk.PUT("/:id", needAPIKey, spaceSdkHandler.Update)
		spaceSdk.DELETE("/:id", needAPIKey, spaceSdkHandler.Delete)
	}

	userProxyHandler, err := handler.NewInternalServiceProxyHandler(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port))
	if err != nil {
		return nil, fmt.Errorf("error creating user proxy handler:%w", err)
	}

	createUserRoutes(apiGroup, needAPIKey, userProxyHandler, userHandler)

	tokenGroup := apiGroup.Group("token")
	{
		tokenGroup.POST("/:app/:token_name", userProxyHandler.ProxyToApi("/api/v1/token/%s/%s", "app", "token_name"))
		tokenGroup.PUT("/:app/:token_name", userProxyHandler.ProxyToApi("/api/v1/token/%s/%s", "app", "token_name"))
		tokenGroup.DELETE("/:app/:token_name", userProxyHandler.ProxyToApi("/api/v1/token/%s/%s", "app", "token_name"))
		// check token info
		tokenGroup.GET("/:token_value", needAPIKey, userProxyHandler.ProxyToApi("/api/v1/token/%s", "token_value"))
	}

	sshKeyHandler, err := handler.NewSSHKeyHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	{
		apiGroup.GET("/user/:username/ssh_keys", sshKeyHandler.Index)
		apiGroup.POST("/user/:username/ssh_keys", sshKeyHandler.Create)
		apiGroup.DELETE("/user/:username/ssh_key/:name", sshKeyHandler.Delete)
	}

	{
		apiGroup.GET("/organizations", userProxyHandler.Proxy)
		apiGroup.POST("/organizations", userProxyHandler.Proxy)
		apiGroup.GET("/organization/:namespace", userProxyHandler.ProxyToApi("/api/v1/organization/%s", "namespace"))
		apiGroup.PUT("/organization/:namespace", userProxyHandler.ProxyToApi("/api/v1/organization/%s", "namespace"))
		apiGroup.DELETE("/organization/:namespace", userProxyHandler.ProxyToApi("/api/v1/organization/%s", "namespace"))
		// Organization models
		apiGroup.GET("/organization/:namespace/models", userProxyHandler.ProxyToApi("/api/v1/organization/%s/models", "namespace"))
		// Organization datasets
		apiGroup.GET("/organization/:namespace/datasets", userProxyHandler.ProxyToApi("/api/v1/organization/%s/datasets", "namespace"))
		apiGroup.GET("/organization/:namespace/codes", userProxyHandler.ProxyToApi("/api/v1/organization/%s/codes", "namespace"))
		apiGroup.GET("/organization/:namespace/spaces", userProxyHandler.ProxyToApi("/api/v1/organization/%s/spaces", "namespace"))
		apiGroup.GET("/organization/:namespace/collections", userProxyHandler.ProxyToApi("/api/v1/organization/%s/collections", "namespace"))
	}

	{
		apiGroup.GET("/organization/:namespace/members", userProxyHandler.ProxyToApi("/api/v1/organization/%s/members", "namespace"))
		apiGroup.POST("/organization/:namespace/members", userProxyHandler.ProxyToApi("/api/v1/organization/%s/members", "namespace"))
		apiGroup.GET("/organization/:namespace/members/:username", userProxyHandler.ProxyToApi("/api/v1/organization/%s/members/%s", "namespace", "username"))
		apiGroup.PUT("/organization/:namespace/members/:username", userProxyHandler.ProxyToApi("/api/v1/organization/%s/members/%s", "namespace", "username"))
		apiGroup.DELETE("/organization/:namespace/members/:username", userProxyHandler.ProxyToApi("/api/v1/organization/%s/members/%s", "namespace", "username"))
	}
	// Tag
	tagCtrl, err := handler.NewTagHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating tag controller:%w", err)
	}
	apiGroup.GET("/tags", tagCtrl.AllTags)
	// apiGroup.POST("/tag", tagCtrl.NewTag)
	// apiGroup.PUT("/tag", tagCtrl.UpdateTag)
	// apiGroup.DELETE("/tag", tagCtrl.DeleteTag)

	// JWT token
	apiGroup.POST("/jwt/token", needAPIKey, userProxyHandler.Proxy)
	apiGroup.GET("/jwt/:token", needAPIKey, userProxyHandler.ProxyToApi("/api/v1/jwt/%s", "token"))

	// callback
	callbackCtrl, err := callback.NewGitCallbackHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating callback controller:%w", err)
	}
	apiGroup.POST("/callback/git", callbackCtrl.Handle)
	apiGroup.GET("/callback/casdoor", userProxyHandler.Proxy)
	// Sensive check
	if config.SensitiveCheck.Enable {
		sensitiveCtrl := handler.NewSensitiveHandler(config)
		apiGroup.POST("/sensitive/text", sensitiveCtrl.Text)
		apiGroup.POST("/sensitive/image", sensitiveCtrl.Image)
	}

	// MirrorSource
	msHandler, err := handler.NewMirrorSourceHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating mirror source controller:%w", err)
	}

	apiGroup.GET("/mirrors", mirrorHandler.Index)
	mirror := apiGroup.Group("/mirror")
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
		collections.POST("", collectionHandler.Create)
		collections.GET("/:id", collectionHandler.GetCollection)
		collections.PUT("/:id", collectionHandler.UpdateCollection)
		collections.DELETE("/:id", collectionHandler.DeleteCollection)
		collections.POST("/:id/repos", collectionHandler.AddRepoToCollection)
		collections.DELETE("/:id/repos", collectionHandler.RemoveRepoFromCollection)
	}

	// cluster infos
	clusterHandler, err := handler.NewClusterHandler(config)
	if err != nil {
		return nil, fmt.Errorf("fail to creating cluster handler: %w", err)
	}
	cluster := apiGroup.Group("/cluster")
	{
		cluster.GET("", clusterHandler.Index)
		cluster.GET("/:id", clusterHandler.GetClusterById)
		cluster.PUT("/:id", needAPIKey, clusterHandler.Update)
	}

	eventHandler, err := handler.NewEventHandler()
	if err != nil {
		return nil, fmt.Errorf("error creating event handler:%w", err)
	}
	event := apiGroup.Group("/events")
	event.POST("", eventHandler.Create)

	runtimeArchHandler, err := handler.NewRuntimeArchitectureHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating runtime framework architecture handler:%w", err)
	}

	createRuntimeFrameworkRoutes(apiGroup, needAPIKey, modelHandler, runtimeArchHandler)

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
		syncGroup.GET("/version/latest", syncHandler.Latest)
		// syncGroup.GET("/version/oldest", syncHandler.Oldest)
		syncGroup.GET("/client_setting", syncClientSettingHandler.Show)
		syncGroup.POST("/client_setting", syncClientSettingHandler.Create)
	}

	accountingHandler, err := handler.NewAccountingHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating accounting handler setting handler:%w", err)
	}

	createAccountRoutes(apiGroup, needAPIKey, accountingHandler)

	recomHandler, err := handler.NewRecomHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating recomHandler,%w", err)
	}
	recomGroup := apiGroup.Group("/recom")
	{
		recomGroup.POST("opweight", needAPIKey, recomHandler.SetOpWeight)
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

	discussionHandler, err := handler.NewDiscussionHandler()
	if err != nil {
		return nil, fmt.Errorf("error creating discussion handler:%w", err)
	}
	createDiscussionRoutes(apiGroup, needAPIKey, discussionHandler)

	return r, nil
}

func createModelRoutes(config *config.Config, apiGroup *gin.RouterGroup, needAPIKey gin.HandlerFunc, modelHandler *handler.ModelHandler, repoCommonHandler *handler.RepoHandler) {
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
		modelsGroup.GET("/:namespace/:name/branches", middleware.RepoType(types.ModelRepo), repoCommonHandler.Branches)
		modelsGroup.GET("/:namespace/:name/tags", middleware.RepoType(types.ModelRepo), repoCommonHandler.Tags)
		// update tags of a certain category
		modelsGroup.POST("/:namespace/:name/tags/:category", middleware.RepoType(types.ModelRepo), repoCommonHandler.UpdateTags)
		modelsGroup.GET("/:namespace/:name/last_commit", middleware.RepoType(types.ModelRepo), repoCommonHandler.LastCommit)
		modelsGroup.GET("/:namespace/:name/commit/:commit_id", middleware.RepoType(types.ModelRepo), repoCommonHandler.CommitWithDiff)
		modelsGroup.GET("/:namespace/:name/tree", middleware.RepoType(types.ModelRepo), repoCommonHandler.Tree)
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
		// invoke model endpoint to do pediction
		modelsGroup.POST("/:namespace/:name/predict", modelHandler.Predict)

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
		modelsGroup.POST("/:namespace/:name/runtime_framework", needAPIKey, middleware.RepoType(types.ModelRepo), repoCommonHandler.RuntimeFrameworkCreate)
		modelsGroup.PUT("/:namespace/:name/runtime_framework/:id", needAPIKey, middleware.RepoType(types.ModelRepo), repoCommonHandler.RuntimeFrameworkUpdate)
		modelsGroup.DELETE("/:namespace/:name/runtime_framework/:id", needAPIKey, middleware.RepoType(types.ModelRepo), repoCommonHandler.RuntimeFrameworkDelete)
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

func createDatasetRoutes(config *config.Config, apiGroup *gin.RouterGroup, dsHandler *handler.DatasetHandler, repoCommonHandler *handler.RepoHandler) {
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

func createCodeRoutes(config *config.Config, apiGroup *gin.RouterGroup, codeHandler *handler.CodeHandler, repoCommonHandler *handler.RepoHandler) {
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

func createSpaceRoutes(config *config.Config, apiGroup *gin.RouterGroup, spaceHandler *handler.SpaceHandler, repoCommonHandler *handler.RepoHandler) {
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

func createUserRoutes(apiGroup *gin.RouterGroup, needAPIKey gin.HandlerFunc, userProxyHandler *handler.InternalServiceProxyHandler, userHandler *handler.UserHandler) {
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
		// User likes
		apiGroup.PUT("/user/:username/likes/:repo_id", userHandler.LikesAdd)
		apiGroup.DELETE("/user/:username/likes/:repo_id", userHandler.LikesDelete)
		apiGroup.GET("/user/:username/likes/spaces", userHandler.LikesSpaces)
		apiGroup.GET("/user/:username/likes/codes", userHandler.LikesCodes)
		apiGroup.GET("/user/:username/likes/models", userHandler.LikesModels)
		apiGroup.GET("/user/:username/likes/datasets", userHandler.LikesDatasets)
		apiGroup.GET("/user/:username/run/:repo_type", userHandler.GetRunDeploys)
		apiGroup.GET("/user/:username/finetune/instances", userHandler.GetFinetuneInstances)
	}

	// User collection
	apiGroup.GET("/user/:username/collections", userHandler.UserCollections)
	apiGroup.GET("/user/:username/likes/collections", userHandler.LikesCollections)
	apiGroup.PUT("/user/:username/likes/collections/:id", userHandler.LikeCollection)
	apiGroup.DELETE("/user/:username/likes/collections/:id", userHandler.UnLikeCollection)
	// user owned tokens
	apiGroup.GET("/user/:username/tokens", userProxyHandler.ProxyToApi("/api/v1/user/%s/tokens", "username"))

	// serverless list
	apiGroup.GET("/user/:username/run/serverless", needAPIKey, userHandler.GetRunServerless)
}

func createRuntimeFrameworkRoutes(apiGroup *gin.RouterGroup, needAPIKey gin.HandlerFunc, modelHandler *handler.ModelHandler, runtimeArchHandler *handler.RuntimeArchitectureHandler) {
	runtimeFramework := apiGroup.Group("/runtime_framework")
	{
		runtimeFramework.GET("/:id/models", modelHandler.ListByRuntimeFrameworkID)
		runtimeFramework.GET("", modelHandler.ListAllRuntimeFramework)
		runtimeFramework.POST("/:id", needAPIKey, modelHandler.UpdateModelRuntimeFrameworks)
		runtimeFramework.DELETE("/:id", needAPIKey, modelHandler.DeleteModelRuntimeFrameworks)
		runtimeFramework.GET("/models", modelHandler.ListModelsOfRuntimeFrameworks)

		runtimeFramework.GET("/:id/architecture", needAPIKey, runtimeArchHandler.ListByRuntimeFrameworkID)
		runtimeFramework.PUT("/:id/architecture", needAPIKey, runtimeArchHandler.UpdateArchitecture)
		runtimeFramework.DELETE("/:id/architecture", needAPIKey, runtimeArchHandler.DeleteArchitecture)
		runtimeFramework.POST("/:id/scan", needAPIKey, runtimeArchHandler.ScanArchitecture)
	}
}

func createAccountRoutes(apiGroup *gin.RouterGroup, needAPIKey gin.HandlerFunc, accountingHandler *handler.AccountingHandler) {
	accountingGroup := apiGroup.Group("/accounting")
	{
		meterGroup := accountingGroup.Group("/metering")
		{
			meterGroup.GET("/:id/statements", accountingHandler.QueryMeteringStatementByUserID)
		}
	}
}

func createHFRoutes(r *gin.Engine, hfdsHandler *handler.HFDatasetHandler, repoCommonHandler *handler.RepoHandler, modelHandler *handler.ModelHandler, userHandler *handler.UserHandler) {
	// Huggingface SDK routes
	hfGroup := r.Group("/hf")
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
				// compitable with HF model info api, used for sdk like this:  huggingface_hub.model_info(repo_id, revision)
				hfModelAPIGroup.GET("/:namespace/:name/revision/:ref", middleware.RepoMapping(types.ModelRepo), modelHandler.SDKModelInfo)
			}
			hfDSAPIGroup := hfAPIGroup.Group("/datasets")
			{
				// compitable with HF dataset info api, used for sdk like this: huggingface_hub.dataset_info(repo_id, revision)
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
