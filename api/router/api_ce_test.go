//go:build !ee && !saas

package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
)

func TestCreateRepoRoutes_AdminIndustryScanNotRegistered_CE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	apiGroup := engine.Group("/api/v1")

	createRepoRoutes(apiGroup, middleware.MiddlewareCollection{}, &handler.RepoHandler{})

	requireRoute(t, engine.Routes(), http.MethodPost, "/api/v1/models/:namespace/:name/mirror_from_saas")
	assertNoRoute(t, engine.Routes(), http.MethodPost, "/api/v1/admin/:repo_type/:namespace/:name/industry_tags/scan")
}
