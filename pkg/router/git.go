package router

import (
	"net/http"

	"git-devops.opencsg.com/product/community/starhub-server/config"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/dataset"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/api/controller/model"
	"github.com/gin-gonic/gin"
)

type GitHandler interface {
	http.Handler
}

func NewGitHandler(
	config *config.Config,
	modelCtrl *model.Controller,
	datasetCtrl *dataset.Controller,
) GitHandler {
	_ = datasetCtrl
	_ = modelCtrl
	_ = config
	r := gin.Default()
	return r
}
