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

	apiGroup.POST("/datasets", dsHandler.Create)
	apiGroup.GET("/datasets", dsHandler.Index)
	apiGroup.PUT("/datasets/:namespace/:name", dsHandler.Update)
	apiGroup.DELETE("/datasets/:namespace/:name", dsHandler.Delete)
	apiGroup.GET("/datasets/:namespace/:name", dsHandler.Show)
	apiGroup.GET("/datasets/:namespace/:name/branches", dsHandler.Branches)
	apiGroup.GET("/datasets/:namespace/:name/tags", dsHandler.Tags)
	apiGroup.GET("/datasets/:namespace/:name/last_commit", dsHandler.LastCommit)
	apiGroup.GET("/datasets/:namespace/:name/tree", dsHandler.Tree)
	apiGroup.GET("/datasets/:namespace/:name/commits", dsHandler.Commits)
	apiGroup.POST("/datasets/:namespace/:name/raw/*file_path", dsHandler.CreateFile)
	apiGroup.GET("/datasets/:namespace/:name/raw/*file_path", dsHandler.FileRaw)
	apiGroup.GET("/datasets/:namespace/:name/blob/*file_path", dsHandler.FileInfo)
	apiGroup.GET("/datasets/:namespace/:name/download/*file_path", dsHandler.DownloadFile)
	apiGroup.GET("/datasets/:namespace/:name/resolve/*file_path", dsHandler.ResolveDownload)
	apiGroup.PUT("/datasets/:namespace/:name/raw/*file_path", dsHandler.UpdateFile)
	apiGroup.POST("/datasets/:namespace/:name/update_downloads", dsHandler.UpdateDownloads)
	apiGroup.POST("/datasets/:namespace/:name/upload_file", dsHandler.UploadFile)

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
		// invoke model prediction
		spaces.POST("/:namespace/:name/predict", spaceHandler.Predict)
		// depoly and start running the space
		spaces.POST("/:namespace/:name/run", nil)
		// stop running space
		spaces.POST("/:namespace/:name/stop", nil)
		// pull space running status
		spaces.POST("/:namespace/:name/status", nil)
		// call space webhook api
		spaces.POST("/:namespace/:name/webhook", nil)
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
