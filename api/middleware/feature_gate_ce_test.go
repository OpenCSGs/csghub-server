//go:build (!ee && !saas) || license_issuer

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/common/types"
)

func TestNewFeatureGate_Passthrough(t *testing.T) {
	factory, err := NewFeatureGate(nil)

	assert.NoError(t, err)
	assert.NotNil(t, factory)
}

func TestFeatureGate_Passthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(FeatureGate(nil, types.FeatureAuditLog))
	router.GET("/admin/audit_logs", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/audit_logs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
