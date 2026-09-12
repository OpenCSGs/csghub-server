//go:build !ee && !saas

package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/config"
)

func TestCreateRepoRoutes_AdminIndustryScanNotRegistered_CE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	apiGroup := engine.Group("/api/v1")

	createRepoRoutes(apiGroup, middleware.MiddlewareCollection{}, &handler.RepoHandler{})

	requireRoute(t, engine.Routes(), http.MethodPost, "/api/v1/models/:namespace/:name/mirror_from_saas")
	requireRoute(t, engine.Routes(), http.MethodGet, "/api/v1/models/:namespace/:name/mirror_from_saas/status")
	assertNoRoute(t, engine.Routes(), http.MethodPost, "/api/v1/admin/:repo_type/:namespace/:name/industry_tags/scan")
}

// TestAddOrgRoutes_CE verifies CE never enables hierarchy organization routes.
func TestAddOrgRoutes_CE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config := &config.Config{}
	config.Organization.EnableUnit = true
	require.False(t, enableUnit(config))

	engine := gin.New()
	require.NoError(t, addOrgRoutes(engine.Group("/api/v1"), middleware.MiddlewareCollection{}, config))
	assertNoRoute(t, engine.Routes(), http.MethodGet, "/api/v1/:repo_type/:namespace/:name/authorization/inheritance")
	require.Empty(t, engine.Routes())
}
