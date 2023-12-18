package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/api/handler"
	"opencsg.com/starhub-server/api/handler/callback"
	memberHandler "opencsg.com/starhub-server/api/handler/member"
	"opencsg.com/starhub-server/api/middleware"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/component/member"
)

func NewRouter(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	r.Use(middleware.Authenticator(config))
	r.Use(gin.Recovery())
	apiGroup := r.Group("/api/v1")
	//TODO:use middleware to handle common response
	// Models routes
	modelHandler, err := handler.NewModelHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating model controller:%w", err)
	}
	apiGroup.POST("/models", modelHandler.Create)
	apiGroup.GET("/models", modelHandler.Index)
	apiGroup.PUT("/models/:namespace/:name", modelHandler.Update)
	apiGroup.DELETE("/models/:namespace/:name", modelHandler.Delete)
	apiGroup.GET("/models/:namespace/:name/detail", modelHandler.Detail)
	apiGroup.GET("/models/:namespace/:name/branches", modelHandler.Branches)
	apiGroup.GET("/models/:namespace/:name/tags", modelHandler.Tags)
	apiGroup.GET("/models/:namespace/:name/last_commit", modelHandler.LastCommit)
	apiGroup.GET("/models/:namespace/:name/tree", modelHandler.Tree)
	apiGroup.GET("/models/:namespace/:name/commits", modelHandler.Commits)
	apiGroup.GET("/models/:namespace/:name/raw/*file_path", modelHandler.FileRaw)
	apiGroup.POST("/models/:namespace/:name/raw/*file_path", modelHandler.CreateFile)
	apiGroup.PUT("/models/:namespace/:name/raw/*file_path", modelHandler.UpdateFile)

	// Dataset routes
	dsHandler, err := handler.NewDatasetHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dataset handler:%w", err)
	}
	apiGroup.POST("/datasets", dsHandler.Create)
	apiGroup.GET("/datasets", dsHandler.Index)
	apiGroup.PUT("/datasets/:namespace/:name", dsHandler.Update)
	apiGroup.DELETE("/datasets/:namespace/:name", dsHandler.Delete)
	apiGroup.GET("/datasets/:namespace/:name/detail", dsHandler.Detail)
	apiGroup.GET("/datasets/:namespace/:name/branches", dsHandler.Branches)
	apiGroup.GET("/datasets/:namespace/:name/tags", dsHandler.Tags)
	apiGroup.GET("/datasets/:namespace/:name/last_commit", dsHandler.LastCommit)
	apiGroup.GET("/datasets/:namespace/:name/tree", dsHandler.Tree)
	apiGroup.GET("/datasets/:namespace/:name/commits", dsHandler.Commits)
	apiGroup.POST("/datasets/:namespace/:name/raw/*file_path", dsHandler.CreateFile)
	apiGroup.GET("/datasets/:namespace/:name/raw/*file_path", dsHandler.FileRaw)
	apiGroup.PUT("/datasets/:namespace/:name/raw/*file_path", dsHandler.UpdateFile)

	// User routes
	userCtrl, err := handler.NewUserHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	apiGroup.POST("/users", userCtrl.Create)
	apiGroup.PUT("/users/:username", userCtrl.Update)
	// User models
	apiGroup.GET("/user/:username/models", userCtrl.Models)
	// User datasets
	apiGroup.GET("/user/:username/datasets", userCtrl.Datasets)

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
	apiGroup.DELETE("/user/:username/ssh_key/:id", sshKeyHandler.Delete)

	//Organization
	orgHandler, err := handler.NewOrganizationHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	apiGroup.GET("/organizations", orgHandler.Index)
	apiGroup.POST("/organizations", orgHandler.Create)
	apiGroup.PUT("/organizations/:name", orgHandler.Update)
	apiGroup.DELETE("/organizations/:name", orgHandler.Delete)

	//Member
	memberCtrl, err := member.New(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	apiGroup.GET("/organizations/:name/members", memberHandler.HandleIndex(memberCtrl))
	apiGroup.POST("/organizations/:name/members", memberHandler.HandleCreate(memberCtrl))
	apiGroup.PUT("/organizations/:name/members/:username", memberHandler.HandleUpdate(memberCtrl))
	apiGroup.DELETE("/organizations/:name/members/:username", memberHandler.HandleDelete(memberCtrl))

	//Tag
	tagCtrl, err := handler.NewTagHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating tag controller:%w", err)
	}
	apiGroup.GET("/tags", tagCtrl.AllTags)
	// apiGroup.POST("/tag", tagCtrl.NewTag)
	// apiGroup.PUT("/tag", tagCtrl.UpdateTag)
	// apiGroup.DELETE("/tag", tagCtrl.DeleteTag)

	//callback
	callbackCtrl, err := callback.NewGitCallbackHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating callback controller:%w", err)
	}
	apiGroup.POST("/callback/git", callbackCtrl.Handle)
	return r, nil
}
