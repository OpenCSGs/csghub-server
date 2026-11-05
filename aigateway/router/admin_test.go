//go:build !saas

package router

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/handler"
	"opencsg.com/csghub-server/api/middleware"
)

// TestSubscriptionModelRoutesUnavailableOutsideSaaS keeps the edition boundary of token plan administration.
func TestSubscriptionModelRoutesUnavailableOutsideSaaS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	createAdminTokenPlanModelRoutes(engine.Group("/api/v1/admin"), middleware.MiddlewareCollection{}, &handler.OpenAIHandlerImpl{}, nil)
	require.Empty(t, engine.Routes())
}
