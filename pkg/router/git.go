package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/config"
	"opencsg.com/starhub-server/pkg/api/controller/dataset"
	"opencsg.com/starhub-server/pkg/api/controller/model"
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
