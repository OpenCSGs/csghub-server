package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/component/dataset"
	"opencsg.com/starhub-server/component/model"
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
