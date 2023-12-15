package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/component"
)

type GitHandler interface {
	http.Handler
}

func NewGitHandler(
	config *config.Config,
	modelCtrl *component.ModelComponent,
	datasetCtrl *component.DatasetComponent,
) GitHandler {
	_ = datasetCtrl
	_ = modelCtrl
	_ = config
	r := gin.Default()
	return r
}
