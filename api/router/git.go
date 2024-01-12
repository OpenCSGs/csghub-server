package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
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
