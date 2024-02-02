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

	if enableSwagger {
		r.GET("/api/v1/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	r.Use(middleware.Authenticator(config))
	r.Use(gin.Recovery())
	r.Use(middleware.Log())
	apiGroup := r.Group("/api/v1")
	// TODO:use middleware to handle common response
	// Models routes
	modelHandler, err := handler.NewModelHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating model controller:%w", err)
	}
	apiGroup.POST("/models", modelHandler.Create)
	apiGroup.GET("/models", modelHandler.Index)
	apiGroup.PUT("/models/:namespace/:name", modelHandler.Update)
	apiGroup.DELETE("/models/:namespace/:name", modelHandler.Delete)
	apiGroup.GET("/models/:namespace/:name", modelHandler.Show)
	apiGroup.GET("/models/:namespace/:name/detail", modelHandler.Detail)
	apiGroup.GET("/models/:namespace/:name/branches", modelHandler.Branches)
	apiGroup.GET("/models/:namespace/:name/tags", modelHandler.Tags)
	apiGroup.GET("/models/:namespace/:name/last_commit", modelHandler.LastCommit)
	apiGroup.GET("/models/:namespace/:name/tree", modelHandler.Tree)
	apiGroup.GET("/models/:namespace/:name/commits", modelHandler.Commits)
	apiGroup.GET("/models/:namespace/:name/raw/*file_path", modelHandler.FileRaw)
	apiGroup.GET("/models/:namespace/:name/download/*file_path", modelHandler.DownloadFile)
	apiGroup.POST("/models/:namespace/:name/raw/*file_path", modelHandler.CreateFile)
	apiGroup.PUT("/models/:namespace/:name/raw/*file_path", modelHandler.UpdateFile)
	apiGroup.POST("/models/:namespace/:name/update_downloads", modelHandler.UpdateDownloads)

	// Dataset routes
	dsHandler, err := handler.NewDatasetHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dataset handler:%w", err)
	}
	apiGroup.POST("/datasets", dsHandler.Create)
	apiGroup.GET("/datasets", dsHandler.Index)
	apiGroup.PUT("/datasets/:namespace/:name", dsHandler.Update)
	apiGroup.DELETE("/datasets/:namespace/:name", dsHandler.Delete)
	apiGroup.GET("/datasets/:namespace/:name", dsHandler.Show)
	apiGroup.GET("/datasets/:namespace/:name/detail", dsHandler.Detail)
	apiGroup.GET("/datasets/:namespace/:name/branches", dsHandler.Branches)
	apiGroup.GET("/datasets/:namespace/:name/tags", dsHandler.Tags)
	apiGroup.GET("/datasets/:namespace/:name/last_commit", dsHandler.LastCommit)
	apiGroup.GET("/datasets/:namespace/:name/tree", dsHandler.Tree)
	apiGroup.GET("/datasets/:namespace/:name/commits", dsHandler.Commits)
	apiGroup.POST("/datasets/:namespace/:name/raw/*file_path", dsHandler.CreateFile)
	apiGroup.GET("/datasets/:namespace/:name/raw/*file_path", dsHandler.FileRaw)
	apiGroup.GET("/datasets/:namespace/:name/download/*file_path", dsHandler.DownloadFile)
	apiGroup.PUT("/datasets/:namespace/:name/raw/*file_path", dsHandler.UpdateFile)
	apiGroup.POST("/datasets/:namespace/:name/update_downloads", dsHandler.UpdateDownloads)

	// Dataset viewer
	// dsViewerHandler, err := handler.NewDatasetViewerHandler(config)
	// if err != nil {
	// 	return nil, fmt.Errorf("error creating dataset viewer handler:%w", err)
	// }
	// apiGroup.GET("/datasets/:namespace/:name/viewer/*file_path", dsViewerHandler.View)

	spaceHandler, err := handler.NewSpaceHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating space handler:%w", err)
	}
	spaces := apiGroup.Group("/spaces")
	{
		// list all spaces
		spaces.GET("/", spaceHandler.Index)
		spaces.POST("/", spaceHandler.Create)
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
