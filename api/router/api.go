package router

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/api/handler"
	acHandler "opencsg.com/starhub-server/api/handler/accesstoken"
	datasetHandler "opencsg.com/starhub-server/api/handler/dataset"
	memberHandler "opencsg.com/starhub-server/api/handler/member"
	modelHandler "opencsg.com/starhub-server/api/handler/model"
	orgHandler "opencsg.com/starhub-server/api/handler/organization"
	sshKeyHandler "opencsg.com/starhub-server/api/handler/sshkey"
	userHandler "opencsg.com/starhub-server/api/handler/user"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/component/accesstoken"
	"opencsg.com/starhub-server/component/dataset"
	"opencsg.com/starhub-server/component/member"
	"opencsg.com/starhub-server/component/model"
	"opencsg.com/starhub-server/component/organization"
	"opencsg.com/starhub-server/component/sshkey"
	"opencsg.com/starhub-server/component/user"
)

func NewRouter(config *config.Config) (*gin.Engine, error) {
	r := gin.New()
	apiGroup := r.Group("/api/v1")
	//TODO:use middleware to handle common response
	// Models routes
	modelCtrl, err := model.New(config)
	if err != nil {
		return nil, fmt.Errorf("error creating model controller:%w", err)
	}
	apiGroup.POST("/models", modelHandler.HandleCreate(modelCtrl))
	apiGroup.GET("/models", modelHandler.HandleIndex(modelCtrl))
	apiGroup.PUT("/models/:namespace/:name", modelHandler.HandleUpdate(modelCtrl))
	apiGroup.DELETE("/models/:namespace/:name", modelHandler.HandleDelete(modelCtrl))
	apiGroup.GET("/models/:namespace/:name/detail", modelHandler.HandleDetail(modelCtrl))
	apiGroup.GET("/models/:namespace/:name/branches", modelHandler.HandleBranches(modelCtrl))
	apiGroup.GET("/models/:namespace/:name/tags", modelHandler.HandleTags(modelCtrl))
	apiGroup.GET("/models/:namespace/:name/last_commit", modelHandler.HandleLastCommit(modelCtrl))
	apiGroup.GET("/models/:namespace/:name/tree", modelHandler.HandleTree(modelCtrl))
	apiGroup.GET("/models/:namespace/:name/commits", modelHandler.HandleCommits(modelCtrl))
	apiGroup.GET("/models/:namespace/:name/raw/*file_path", modelHandler.HandleFileRaw(modelCtrl))
	apiGroup.POST("/models/:namespace/:name/raw/*file_path", modelHandler.HandleFileCreate(modelCtrl))
	apiGroup.PUT("/models/:namespace/:name/raw/*file_path", modelHandler.HandleFileUpdate(modelCtrl))

	// Dataset routes
	datasetCtrl, err := dataset.New(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dataset controller:%w", err)
	}
	apiGroup.POST("/datasets", datasetHandler.HandleCreate(datasetCtrl))
	apiGroup.GET("/datasets", datasetHandler.HandleIndex(datasetCtrl))
	apiGroup.PUT("/datasets/:namespace/:name", datasetHandler.HandleUpdate(datasetCtrl))
	apiGroup.DELETE("/datasets/:namespace/:name", datasetHandler.HandleDelete(datasetCtrl))
	apiGroup.GET("/datasets/:namespace/:name/detail", datasetHandler.HandleDetail(datasetCtrl))
	apiGroup.GET("/datasets/:namespace/:name/branches", datasetHandler.HandleBranches(datasetCtrl))
	apiGroup.GET("/datasets/:namespace/:name/tags", datasetHandler.HandleTags(datasetCtrl))
	apiGroup.GET("/datasets/:namespace/:name/last_commit", datasetHandler.HandleLastCommit(datasetCtrl))
	apiGroup.GET("/datasets/:namespace/:name/tree", datasetHandler.HandleTree(datasetCtrl))
	apiGroup.GET("/datasets/:namespace/:name/commits", datasetHandler.HandleCommits(datasetCtrl))
	apiGroup.GET("/datasets/:namespace/:name/raw/*file_path", datasetHandler.HandleFileRaw(datasetCtrl))
	apiGroup.PUT("/datasets/:namespace/:name/raw/*file_path", datasetHandler.HandleFileUpdate(datasetCtrl))

	dsHandler, err := handler.NewDatasetHandler(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dataset handler:%w", err)
	}
	apiGroup.POST("/datasets/:namespace/:name/raw/*file_path", dsHandler.CreateFile)

	// User routes
	userCtrl, err := user.New(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	apiGroup.POST("/users", userHandler.HandleCreate(userCtrl))
	apiGroup.PUT("/users/", userHandler.HandleUpdate(userCtrl))

	acCtrl, err := accesstoken.New(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	apiGroup.POST("/user/:username/tokens", acHandler.HandleCreate(acCtrl))
	apiGroup.DELETE("/user/:username/tokens/:token_name", acHandler.HandleDelete(acCtrl))

	sshKeyCtrl, err := sshkey.New(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	apiGroup.GET("/user/:username/ssh_keys", sshKeyHandler.HandleIndex(sshKeyCtrl))
	apiGroup.POST("/user/:username/ssh_keys", sshKeyHandler.HandleCreate(sshKeyCtrl))
	apiGroup.DELETE("/user/:username/ssh_key/:id", sshKeyHandler.HandleDelete(sshKeyCtrl))

	// User models
	apiGroup.GET("/user/:username/models", userHandler.HandleModels(userCtrl))

	// User datasets
	apiGroup.GET("/user/:username/datasets", userHandler.HandleDatasets(userCtrl))

	//Organization
	orgCtrl, err := organization.New(config)
	if err != nil {
		return nil, fmt.Errorf("error creating user controller:%w", err)
	}
	apiGroup.GET("/organizations", orgHandler.HandleIndex(orgCtrl))
	apiGroup.POST("/organizations", orgHandler.HandleCreate(orgCtrl))
	apiGroup.PUT("/organizations/:name", orgHandler.HandleUpdate(orgCtrl))
	apiGroup.DELETE("/organizations/:name", orgHandler.HandleDelete(orgCtrl))

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

	return r, nil
}
