package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/config"
	"opencsg.com/starhub-server/pkg/api/controller/accesstoken"
	"opencsg.com/starhub-server/pkg/api/controller/dataset"
	"opencsg.com/starhub-server/pkg/api/controller/member"
	"opencsg.com/starhub-server/pkg/api/controller/model"
	"opencsg.com/starhub-server/pkg/api/controller/organization"
	"opencsg.com/starhub-server/pkg/api/controller/sshkey"
	"opencsg.com/starhub-server/pkg/api/controller/user"
	acHandler "opencsg.com/starhub-server/pkg/api/handler/accesstoken"
	datasetHandler "opencsg.com/starhub-server/pkg/api/handler/dataset"
	memberHandler "opencsg.com/starhub-server/pkg/api/handler/member"
	modelHandler "opencsg.com/starhub-server/pkg/api/handler/model"
	orgHandler "opencsg.com/starhub-server/pkg/api/handler/organization"
	sshKeyHandler "opencsg.com/starhub-server/pkg/api/handler/sshkey"
	userHandler "opencsg.com/starhub-server/pkg/api/handler/user"
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
	orgCtrl *organization.Controller,
	memberCtrl *member.Controller,
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

	//Organization
	apiGroup.GET("/organizations", orgHandler.HandleIndex(orgCtrl))
	apiGroup.POST("/organizations", orgHandler.HandleCreate(orgCtrl))
	apiGroup.PUT("/organizations/:name", orgHandler.HandleUpdate(orgCtrl))
	apiGroup.DELETE("/organizations/:name", orgHandler.HandleDelete(orgCtrl))

	//Member
	apiGroup.GET("/organizations/:name/members", memberHandler.HandleIndex(memberCtrl))
	apiGroup.POST("/organizations/:name/members", memberHandler.HandleCreate(memberCtrl))
	apiGroup.PUT("/organizations/:name/members/:username", memberHandler.HandleUpdate(memberCtrl))
	apiGroup.DELETE("/organizations/:name/members/:username", memberHandler.HandleDelete(memberCtrl))

	return r
}
