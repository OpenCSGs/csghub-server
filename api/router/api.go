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

	// Huggingface SDK routes
	hfGroup := r.Group("/hf")
	{
		hfGroup.GET("/:namespace/:name/resolve/:branch/*file_path", middleware.RepoType(types.ModelRepo), repoCommonHandler.SDKDownload)
		hfGroup.HEAD("/:namespace/:name/resolve/:branch/*file_path", middleware.RepoType(types.ModelRepo), repoCommonHandler.HeadSDKDownload)
		hfGroup.GET("/datasets/:namespace/:name/resolve/:branch/*file_path", middleware.RepoType(types.DatasetRepo), repoCommonHandler.SDKDownload)
		hfGroup.HEAD("/datasets/:namespace/:name/resolve/:branch/*file_path", middleware.RepoType(types.DatasetRepo), repoCommonHandler.HeadSDKDownload)
		hfAPIGroup := hfGroup.Group("/api")
		{
			// compitable with HF model info api, used for sdk like this:  huggingface_hub.model_info(repo_id, revision)
			hfAPIGroup.GET("/models/:namespace/:name/revision/:ref", modelHandler.SDKModelInfo)
			// compitable with HF dataset info api, used for sdk like this: huggingface_hub.dataset_info(repo_id, revision)
			hfAPIGroup.GET("/datasets/:namespace/:name/revision/:ref", middleware.RepoType(types.DatasetRepo), repoCommonHandler.SDKListFiles)
			hfAPIGroup.GET("/whoami-v2", userHandler.UserPermission)
		}
	}

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
	// Huggingface SDK routes
	modelHandler, err = handler.NewModelHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating model controller:%w", err)
	}
	dsHandler, err = handler.NewDatasetHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dataset handler:%w", err)
	}

	// Models routes
	modelsGroup := apiGroup.Group("/models")
	{
		modelsGroup.POST("", modelHandler.Create)
		modelsGroup.GET("", modelHandler.Index)
		modelsGroup.PUT("/:namespace/:name", modelHandler.Update)
		modelsGroup.DELETE("/:namespace/:name", modelHandler.Delete)
		modelsGroup.GET("/:namespace/:name", modelHandler.Show)
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
			modelsGroup.POST("/:namespace/:name/mirror_from_saas/sync", middleware.RepoType(types.ModelRepo), repoCommonHandler.MirrorFromSaasSync)
		}

		// runtime framework
		modelsGroup.GET("/:namespace/:name/runtime_framework", middleware.RepoType(types.ModelRepo), repoCommonHandler.RuntimeFrameworkList)
		modelsGroup.POST("/:namespace/:name/runtime_framework", middleware.RepoType(types.ModelRepo), repoCommonHandler.RuntimeFrameworkCreate)
		modelsGroup.PUT("/:namespace/:name/runtime_framework/:id", middleware.RepoType(types.ModelRepo), repoCommonHandler.RuntimeFrameworkUpdate)
		modelsGroup.DELETE("/:namespace/:name/runtime_framework/:id", middleware.RepoType(types.ModelRepo), repoCommonHandler.RuntimeFrameworkDelete)
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

	}

	// Dataset routes
	datasetsGroup := apiGroup.Group("/datasets")
	{
		datasetsGroup.POST("", dsHandler.Create)
		datasetsGroup.GET("", dsHandler.Index)
		datasetsGroup.PUT("/:namespace/:name", dsHandler.Update)
		datasetsGroup.DELETE("/:namespace/:name", dsHandler.Delete)
		datasetsGroup.GET("/:namespace/:name", dsHandler.Show)
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
			datasetsGroup.POST("/:namespace/:name/mirror_from_saas/sync", middleware.RepoType(types.DatasetRepo), repoCommonHandler.MirrorFromSaasSync)
		}
	}

	// Code routes
	codeHandler, err := handler.NewCodeHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating code handler:%w", err)
	}

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
			codesGroup.POST("/:namespace/:name/mirror_from_saas/sync", middleware.RepoType(types.CodeRepo), repoCommonHandler.MirrorFromSaasSync)
		}
	}

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
			spaces.POST("/:namespace/:name/mirror_from_saas/sync", middleware.RepoType(types.SpaceRepo), repoCommonHandler.MirrorFromSaasSync)
		}

		spaces.GET("/:namespace/:name/runtime_framework", middleware.RepoType(types.SpaceRepo), repoCommonHandler.RuntimeFrameworkList)
		spaces.POST("/:namespace/:name/runtime_framework", middleware.RepoType(types.SpaceRepo), repoCommonHandler.RuntimeFrameworkCreate)
		spaces.PUT("/:namespace/:name/runtime_framework/:id", middleware.RepoType(types.SpaceRepo), repoCommonHandler.RuntimeFrameworkUpdate)
		spaces.DELETE("/:namespace/:name/runtime_framework/:id", middleware.RepoType(types.SpaceRepo), repoCommonHandler.RuntimeFrameworkDelete)
		spaces.GET("/:namespace/:name/run", middleware.RepoType(types.SpaceRepo), repoCommonHandler.DeployList)
		spaces.GET("/:namespace/:name/run/:id", middleware.RepoType(types.SpaceRepo), repoCommonHandler.DeployDetail)
		spaces.GET("/:namespace/:name/run/:id/status", middleware.RepoType(types.SpaceRepo), repoCommonHandler.DeployStatus)
		spaces.GET("/:namespace/:name/run/:id/logs/:instance", middleware.RepoType(types.SpaceRepo), repoCommonHandler.DeployInstanceLogs)
	}

	spaceResourceHandler, err := handler.NewSpaceResourceHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating space resource handler:%w", err)
	}

	spaceResource := apiGroup.Group("space_resources")
	{
		spaceResource.GET("", spaceResourceHandler.Index)
		spaceResource.POST("", spaceResourceHandler.Create)
		spaceResource.PUT("/:id", spaceResourceHandler.Update)
		spaceResource.DELETE("/:id", spaceResourceHandler.Delete)
	}

	spaceSdkHandler, err := handler.NewSpaceSdkHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating space sdk handler:%w", err)
	}

	spaceSdk := apiGroup.Group("space_sdks")
	{
		spaceSdk.GET("", spaceSdkHandler.Index)
		spaceSdk.POST("", spaceSdkHandler.Create)
		spaceSdk.PUT("/:id", spaceSdkHandler.Update)
		spaceSdk.DELETE("/:id", spaceSdkHandler.Delete)
	}

	acHandler, err := handler.NewAccessTokenHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}

	{
		apiGroup.POST("/users", userHandler.Create)
		apiGroup.PUT("/users/:username", userHandler.Update)
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
		//user owned tokens
		apiGroup.GET("/user/:username/tokens", acHandler.GetUserTokens)
	}

	tokenGroup := apiGroup.Group("token")
	{
		tokenGroup.POST("/:app/:token_name", acHandler.CreateAppToken)
		tokenGroup.PUT("/:app/:token_name", acHandler.Refresh)
		tokenGroup.DELETE("/:app/:token_name", acHandler.DeleteAppToken)
		// check token info
		tokenGroup.GET("/:token_value", needAPIKey, acHandler.Get)
	}
	//Depreated:
	{
		apiGroup.POST("/user/:username/tokens", acHandler.Create)
		apiGroup.DELETE("/user/:username/tokens/:token_name", acHandler.Delete)
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
	// Organization
	orgHandler, err := handler.NewOrganizationHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	{
		apiGroup.GET("/organizations", orgHandler.Index)
		apiGroup.POST("/organizations", orgHandler.Create)
		apiGroup.PUT("/organizations/:name", orgHandler.Update)
		apiGroup.DELETE("/organizations/:name", orgHandler.Delete)
		// Organization models
		apiGroup.GET("/organization/:namespace/models", orgHandler.Models)
		// Organization datasets
		apiGroup.GET("/organization/:namespace/datasets", orgHandler.Datasets)
		apiGroup.GET("/organization/:namespace/codes", orgHandler.Codes)
		apiGroup.GET("/organization/:namespace/spaces", orgHandler.Spaces)
	}
	// Member
	memberCtrl, err := handler.NewMemberHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	{
		apiGroup.GET("/organizations/:name/members", memberCtrl.Index)
		apiGroup.POST("/organizations/:name/members", memberCtrl.Create)
		apiGroup.PUT("/organizations/:name/members/:username", memberCtrl.Update)
		apiGroup.DELETE("/organizations/:name/members/:username", memberCtrl.Delete)
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
	jwtCtrl, err := handler.NewJWTHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating jwt token controller:%w", err)
	}
	apiGroup.POST("/jwt/token", needAPIKey, jwtCtrl.Create)

	// callback
	callbackCtrl, err := callback.NewGitCallbackHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating callback controller:%w", err)
	}
	apiGroup.POST("/callback/git", callbackCtrl.Handle)
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

	// Mirror
	mirrorHandler, err := handler.NewMirrorHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating mirror controller:%w", err)
	}

	mirror := apiGroup.Group("/mirror")
	mirror.Use(needAPIKey)
	{
		mirror.GET("/sources", msHandler.Index)
		mirror.POST("/sources", msHandler.Create)
		mirror.PUT("/sources/:id", msHandler.Update)
		mirror.DELETE("/sources/:id", msHandler.Delete)
		mirror.GET("/sources/:id", msHandler.Get)
		mirror.POST("/repo", mirrorHandler.CreateMirrorRepo)
	}

	// cluster infos
	clusterHandler, err := handler.NewClusterHandler(config)
	if err != nil {
		return nil, fmt.Errorf("fail to creating cluster handler: %w", err)
	}
	cluster := apiGroup.Group("/cluster")
	{
		cluster.GET("", clusterHandler.Index)
		cluster.PUT("", clusterHandler.Update)
	}

	eventHandler, err := handler.NewEventHandler()
	if err != nil {
		return nil, fmt.Errorf("error creating event handler:%w", err)
	}
	event := apiGroup.Group("/events")
	event.POST("", eventHandler.Create)

	runtimeFramework := apiGroup.Group("/runtime_framework")
	{
		runtimeFramework.GET("/:id/models", modelHandler.ListByRuntimeFrameworkID)
		runtimeFramework.GET("", modelHandler.ListAllRuntimeFramework)
		runtimeFramework.POST("/:id", modelHandler.UpdateModelRuntimeFrameworks)
		runtimeFramework.DELETE("/:id", modelHandler.DeleteModelRuntimeFrameworks)
	}
	syncHandler, err := handler.NewSyncHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sync handler:%w", err)
	}
	syncGroup := apiGroup.Group("sync")
	{
		syncGroup.GET("/version/latest", syncHandler.Latest)
		// syncGroup.GET("/version/oldest", syncHandler.Oldest)
	}

	syncClientSettingHandler, err := handler.NewSyncClientSettingHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sync client setting handler:%w", err)
	}
	apiGroup.POST("/sync_client_setting", syncClientSettingHandler.Create)

	return r, nil
}
