package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/handler/callback"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

func NewRouter(config *config.Config, enableSwagger bool) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Log())

	if enableSwagger {
		r.GET("/api/v1/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	r.Use(middleware.Authenticator(config))
	apiGroup := r.Group("/api/v1")
	// TODO:use middleware to handle common response

	// List trending models and datasets routes
	listHandler, err := handler.NewListHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creatring list handler: %v", err)
	}
	apiGroup.POST("/list/models_by_path", listHandler.ListModelsByPath)
	apiGroup.POST("/list/datasets_by_path", listHandler.ListDatasetsByPath)

	// Huggingface SDK routes
	modelHandler, err := handler.NewModelHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating model controller:%w", err)
	}
	dsHandler, err := handler.NewDatasetHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dataset handler:%w", err)
	}

	hfGroup := r.Group("/hf")
	{
		hfGroup.GET("/:namespace/:name/resolve/:branch/*file_path", modelHandler.SDKDownload)
		hfGroup.HEAD("/:namespace/:name/resolve/:branch/*file_path", modelHandler.HeadSDKDownload)
		hfGroup.GET("/datasets/:namespace/:name/resolve/:branch/*file_path", dsHandler.SDKDownload)
		hfGroup.HEAD("/datasets/:namespace/:name/resolve/:branch/*file_path", dsHandler.HeadSDKDownload)
		hfAPIGroup := hfGroup.Group("/api")
		{
			hfAPIGroup.GET("/models/:namespace/:name/revision/:branch", modelHandler.SDKListFiles)
			hfAPIGroup.GET("/datasets/:namespace/:name/revision/:branch", dsHandler.SDKListFiles)
		}
	}
	// Models routes
	modelsGroup := apiGroup.Group("/models")
	{
		modelsGroup.POST("", modelHandler.Create)
		modelsGroup.GET("", modelHandler.Index)
		modelsGroup.PUT("/:namespace/:name", modelHandler.Update)
		modelsGroup.DELETE("/:namespace/:name", modelHandler.Delete)
		modelsGroup.GET("/:namespace/:name", modelHandler.Show)
		modelsGroup.GET("/:namespace/:name/branches", modelHandler.Branches)
		modelsGroup.GET("/:namespace/:name/tags", modelHandler.Tags)
		modelsGroup.GET("/:namespace/:name/last_commit", modelHandler.LastCommit)
		modelsGroup.GET("/:namespace/:name/tree", modelHandler.Tree)
		modelsGroup.GET("/:namespace/:name/commits", modelHandler.Commits)
		modelsGroup.GET("/:namespace/:name/raw/*file_path", modelHandler.FileRaw)
		modelsGroup.GET("/:namespace/:name/blob/*file_path", modelHandler.FileInfo)
		// The DownloadFile method differs from the SDKDownload interface in a few ways

		// 1.When passing the file_path parameter to the SDKDownload method,
		// it only needs to pass the path of the file itself,
		// whether it is an lfs file or a non-lfs file.
		// The DownloadFile has a different file_path format for lfs files and non-lfs files,
		// and an lfs parameter needs to be added.
		// 2. DownloadFile returns an object store url for lfs files, while SDKDownload redirects directly.
		modelsGroup.GET("/:namespace/:name/download/*file_path", modelHandler.DownloadFile)
		modelsGroup.GET("/:namespace/:name/resolve/*file_path", modelHandler.ResolveDownload)
		modelsGroup.POST("/:namespace/:name/raw/*file_path", modelHandler.CreateFile)
		modelsGroup.PUT("/:namespace/:name/raw/*file_path", modelHandler.UpdateFile)
		modelsGroup.POST("/:namespace/:name/update_downloads", modelHandler.UpdateDownloads)
		modelsGroup.POST("/:namespace/:name/upload_file", modelHandler.UploadFile)
		// invoke model endpoint to do pediction
		modelsGroup.POST("/:namespace/:name/predict", modelHandler.Predict)
	}

	// Dataset routes
	datasetsGroup := apiGroup.Group("/datasets")

	{
		datasetsGroup.POST("", dsHandler.Create)
		datasetsGroup.GET("", dsHandler.Index)
		datasetsGroup.PUT("/:namespace/:name", dsHandler.Update)
		datasetsGroup.DELETE("/:namespace/:name", dsHandler.Delete)
		datasetsGroup.GET("/:namespace/:name", dsHandler.Show)
		datasetsGroup.GET("/:namespace/:name/branches", dsHandler.Branches)
		datasetsGroup.GET("/:namespace/:name/tags", dsHandler.Tags)
		datasetsGroup.GET("/:namespace/:name/last_commit", dsHandler.LastCommit)
		datasetsGroup.GET("/:namespace/:name/tree", dsHandler.Tree)
		datasetsGroup.GET("/:namespace/:name/commits", dsHandler.Commits)
		datasetsGroup.POST("/:namespace/:name/raw/*file_path", dsHandler.CreateFile)
		datasetsGroup.GET("/:namespace/:name/raw/*file_path", dsHandler.FileRaw)
		datasetsGroup.GET("/:namespace/:name/blob/*file_path", dsHandler.FileInfo)
		datasetsGroup.GET("/:namespace/:name/download/*file_path", dsHandler.DownloadFile)
		datasetsGroup.GET("/:namespace/:name/resolve/*file_path", dsHandler.ResolveDownload)
		datasetsGroup.PUT("/:namespace/:name/raw/*file_path", dsHandler.UpdateFile)
		datasetsGroup.POST("/:namespace/:name/update_downloads", dsHandler.UpdateDownloads)
		datasetsGroup.POST("/:namespace/:name/upload_file", dsHandler.UploadFile)
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
		codesGroup.GET("/:namespace/:name/branches", codeHandler.Branches)
		codesGroup.GET("/:namespace/:name/tags", codeHandler.Tags)
		codesGroup.GET("/:namespace/:name/last_commit", codeHandler.LastCommit)
		codesGroup.GET("/:namespace/:name/tree", codeHandler.Tree)
		codesGroup.GET("/:namespace/:name/commits", codeHandler.Commits)
		codesGroup.POST("/:namespace/:name/raw/*file_path", codeHandler.CreateFile)
		codesGroup.GET("/:namespace/:name/raw/*file_path", codeHandler.FileRaw)
		codesGroup.GET("/:namespace/:name/blob/*file_path", codeHandler.FileInfo)
		codesGroup.GET("/:namespace/:name/download/*file_path", codeHandler.DownloadFile)
		codesGroup.GET("/:namespace/:name/resolve/*file_path", codeHandler.ResolveDownload)
		codesGroup.PUT("/:namespace/:name/raw/*file_path", codeHandler.UpdateFile)
		codesGroup.POST("/:namespace/:name/update_downloads", codeHandler.UpdateDownloads)
		codesGroup.POST("/:namespace/:name/upload_file", codeHandler.UploadFile)
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
		spaces.GET("/:namespace/:name", spaceHandler.Get)
		spaces.PUT("/:namespace/:name", spaceHandler.Update)
		spaces.DELETE("/:namespace/:name", spaceHandler.Delete)
		// proxy any request to space api
		spaces.Any("/:namespace/:name/api/*api_name", spaceHandler.Proxy)
		// depoly and start running the space
		spaces.POST("/:namespace/:name/run", nil)
		// stop running space
		spaces.POST("/:namespace/:name/stop", nil)
		// pull space running status
		spaces.POST("/:namespace/:name/status", nil)
		// call space webhook api
		spaces.POST("/:namespace/:name/webhook", nil)

		spaces.GET("/:namespace/:name/branches", spaceHandler.Branches)
		spaces.GET("/:namespace/:name/tags", spaceHandler.Tags)
		spaces.GET("/:namespace/:name/last_commit", spaceHandler.LastCommit)
		spaces.GET("/:namespace/:name/tree", spaceHandler.Tree)
		spaces.GET("/:namespace/:name/commits", spaceHandler.Commits)
		spaces.POST("/:namespace/:name/raw/*file_path", spaceHandler.CreateFile)
		spaces.GET("/:namespace/:name/raw/*file_path", spaceHandler.FileRaw)
		spaces.GET("/:namespace/:name/blob/*file_path", spaceHandler.FileInfo)
		spaces.GET("/:namespace/:name/download/*file_path", spaceHandler.DownloadFile)
		spaces.GET("/:namespace/:name/resolve/*file_path", spaceHandler.ResolveDownload)
		spaces.PUT("/:namespace/:name/raw/*file_path", spaceHandler.UpdateFile)
		spaces.POST("/:namespace/:name/update_downloads", spaceHandler.UpdateDownloads)
		spaces.POST("/:namespace/:name/upload_file", spaceHandler.UploadFile)
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

	// User routes
	userHandler, err := handler.NewUserHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	apiGroup.POST("/users", userHandler.Create)
	apiGroup.PUT("/users/:username", userHandler.Update)
	// User models
	apiGroup.GET("/user/:username/models", userHandler.Models)
	// User datasets
	apiGroup.GET("/user/:username/datasets", userHandler.Datasets)

	acHandler, err := handler.NewAccessTokenHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	apiGroup.POST("/user/:username/tokens", acHandler.Create)
	apiGroup.DELETE("/user/:username/tokens/:token_name", acHandler.Delete)

	sshKeyHandler, err := handler.NewSSHKeyHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	apiGroup.GET("/user/:username/ssh_keys", sshKeyHandler.Index)
	apiGroup.POST("/user/:username/ssh_keys", sshKeyHandler.Create)
	apiGroup.DELETE("/user/:username/ssh_key/:name", sshKeyHandler.Delete)

	// Organization
	orgHandler, err := handler.NewOrganizationHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	apiGroup.GET("/organizations", orgHandler.Index)
	apiGroup.POST("/organizations", orgHandler.Create)
	apiGroup.PUT("/organizations/:name", orgHandler.Update)
	apiGroup.DELETE("/organizations/:name", orgHandler.Delete)
	// Organization models
	apiGroup.GET("/organization/:namespace/models", orgHandler.Models)
	// Organization datasets
	apiGroup.GET("/organization/:namespace/datasets", orgHandler.Datasets)

	// Member
	memberCtrl, err := handler.NewMemberHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	apiGroup.GET("/organizations/:name/members", memberCtrl.Index)
	apiGroup.POST("/organizations/:name/members", memberCtrl.Create)
	apiGroup.PUT("/organizations/:name/members/:username", memberCtrl.Update)
	apiGroup.DELETE("/organizations/:name/members/:username", memberCtrl.Delete)

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
	apiGroup.POST("/jwt/token", jwtCtrl.Create)

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
	return r, nil
}
