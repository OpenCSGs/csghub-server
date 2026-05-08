//go:build !ee && !saas

package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
)

//nolint:unused
func initServerlessBenchmarkHooks() {}

func registerServerlessBenchmarkRoutes(
	modelsServerlessGroup *gin.RouterGroup,
	middlewareCollection middleware.MiddlewareCollection,
	repoCommonHandler *handler.RepoHandler,
) {
}
