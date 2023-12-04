package router

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/config"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/accesstoken"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/sshkey"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/user"
	acHandler "git-devops.opencsg.com/product/community/starhub-server/pkg/api/handler/accesstoken"
	datasetHandler "git-devops.opencsg.com/product/community/starhub-server/pkg/api/handler/dataset"
	modelHandler "git-devops.opencsg.com/product/community/starhub-server/pkg/api/handler/model"
	sshKeyHandler "git-devops.opencsg.com/product/community/starhub-server/pkg/api/handler/sshkey"
	userHandler "git-devops.opencsg.com/product/community/starhub-server/pkg/api/handler/user"
	"github.com/gin-gonic/gin"
)

type APIHandler interface {
	http.Handler
}

func NewAPIHandler(
	config *config.Config,
	modelCtrl *model.Controller,
	datasetCtrl *dataset.Controller,
	userCtrl *user.Controller,
	acCtrl *accesstoken.Controller,
	sshKeyCtrl *sshkey.Controller,
) APIHandler {
	_ = config
	r := gin.New()
	apiGroup := r.Group("/api/v1")
	// Models routes
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

	// Dataset routes
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

	// User routes
	apiGroup.POST("/users", userHandler.HandleCreate(userCtrl))
	apiGroup.PUT("/users/", userHandler.HandleUpdate(userCtrl))
	apiGroup.POST("/user/:username/tokens", acHandler.HandleCreate(acCtrl))
	apiGroup.DELETE("/user/:username/tokens/:token_name", acHandler.HandleDelete(acCtrl))
	apiGroup.GET("/user/:username/ssh_keys", sshKeyHandler.HandleIndex(sshKeyCtrl))
	apiGroup.POST("/user/:username/ssh_keys", sshKeyHandler.HandleCreate(sshKeyCtrl))
	apiGroup.DELETE("/user/:username/ssh_key/:id", sshKeyHandler.HandleDelete(sshKeyCtrl))

	// User models
	apiGroup.GET("/user/:username/models", userHandler.HandleModels(userCtrl))

	// User datasets
	apiGroup.GET("/user/:username/datasets", userHandler.HandleDatasets(userCtrl))
	return r
}
