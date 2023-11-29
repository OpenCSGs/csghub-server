package router

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/config"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/user"
	datasetHandler "git-devops.opencsg.com/product/community/starhub-server/pkg/api/handler/dataset"
	modelHandler "git-devops.opencsg.com/product/community/starhub-server/pkg/api/handler/model"
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
) APIHandler {
	_ = config
	r := gin.New()
	apiGroup := r.Group("/api/v1")
	apiGroup.POST("/models", modelHandler.HandleCreate(modelCtrl))
	apiGroup.POST("/datasets", datasetHandler.HandleCreate(datasetCtrl))
	apiGroup.GET("/models", modelHandler.HandleIndex(modelCtrl))
	apiGroup.GET("/datasets", datasetHandler.HandleIndex(datasetCtrl))
	apiGroup.PUT("/models/:namespace/:name", modelHandler.HandleUpdate(modelCtrl))
	apiGroup.PUT("/datasets/:namespace/:name", datasetHandler.HandleUpdate(datasetCtrl))
	apiGroup.GET("/models/:namespace/:name/detail", modelHandler.HandleDetail(modelCtrl))
	apiGroup.GET("/datasets/:namespace/:name/detail", datasetHandler.HandleDetail(datasetCtrl))
	apiGroup.GET("/models/:namespace/:name/branches", modelHandler.HandleBranches(modelCtrl))
	apiGroup.GET("/datasets/:namespace/:name/branches", datasetHandler.HandleBranches(datasetCtrl))
	apiGroup.GET("/models/:namespace/:name/tags", modelHandler.HandleTags(modelCtrl))
	apiGroup.GET("/datasets/:namespace/:name/tags", datasetHandler.HandleTags(datasetCtrl))
	apiGroup.GET("/models/:namespace/:name/last_commit", modelHandler.HandleLastCommit(modelCtrl))
	apiGroup.GET("/datasets/:namespace/:name/last_commit", datasetHandler.HandleLastCommit(datasetCtrl))
	apiGroup.GET("/models/:namespace/:name/tree", modelHandler.HandleTree(modelCtrl))
	apiGroup.GET("/datasets/:namespace/:name/tree", datasetHandler.HandleTree(datasetCtrl))
	apiGroup.GET("/models/:namespace/:name/commits", modelHandler.HandleCommits(modelCtrl))
	apiGroup.GET("/datasets/:namespace/:name/commits", datasetHandler.HandleCommits(datasetCtrl))
	apiGroup.GET("/models/:namespace/:name/raw/*file_path", modelHandler.HandleFileRaw(modelCtrl))
	apiGroup.GET("/datasets/:namespace/:name/raw/*file_path", datasetHandler.HandleFileRaw(datasetCtrl))
	apiGroup.POST("/users", userHandler.HandleCreate(userCtrl))
	apiGroup.PUT("/users/", userHandler.HandleUpdate(userCtrl))
	return r
}
