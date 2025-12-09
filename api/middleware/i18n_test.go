package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/i18n"
)

func TestLocalizedErrorMiddleware(t *testing.T) {
	// Set up test LocalizerMap
	i18n.InitLocalizersFromEmbedFile()

	// Create a test router
	router := gin.Default()
	router.Use(LocalizedErrorMiddleware())

	// Test case 1: Skipped route
	router.GET("/healthz", func(c *gin.Context) {
		httpbase.BadRequest(c, "Health check failed")
	})

	// Test case 2: Normal response (status code < 400)
	router.GET("/success", func(c *gin.Context) {
		httpbase.OK(c, gin.H{"message": "success"})
	})

	// Test case 3: Error response without body
	router.GET("/empty-error", func(c *gin.Context) {
		c.Status(http.StatusBadRequest)
	})

	// Test case 4: Error response with invalid JSON
	router.GET("/invalid-json", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusBadRequest)
		_, _ = c.Writer.Write([]byte("invalid json"))
	})

	// Test case 5: Error response with invalid error code
	router.GET("/invalid-error-code", func(c *gin.Context) {
		httpbase.BadRequestWithExt(c, errorx.NewCustomError("INVALID", 1, nil, nil))
	})

	// Test case 6: Error response with valid error code, no context
	router.GET("/valid-error-code", func(c *gin.Context) {
		httpbase.BadRequestWithExt(c, errorx.NewCustomError("AUTH", 1, nil, nil))
	})

	// Test case 7: Error response with valid error code, with context
	router.GET("/valid-error-code-with-context", func(c *gin.Context) {
		httpbase.BadRequestWithExt(c, errorx.NewCustomError("REQ", 1, nil, errorx.Ctx().Set("param", "test")))
	})

	// Run tests
	t.Run("SkipRoute", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/healthz", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp httpbase.R
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "Health check failed", resp.Msg)
	})

	t.Run("NormalResponse", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/success", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp httpbase.R
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "OK", resp.Msg)
	})

	t.Run("EmptyErrorResponse", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/empty-error", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		_ = assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, w.Body.Bytes())
	})

	t.Run("InvalidJSONResponse", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/invalid-json", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		_ = assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "invalid json", w.Body.String())
	})

	t.Run("InvalidErrorCode", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/invalid-error-code", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp httpbase.R
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "INVALID-1", resp.Msg)
	})

	t.Run("ValidErrorCode", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/valid-error-code", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp httpbase.R
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "AUTH-1", resp.Msg)
	})

	t.Run("ValidErrorCodeWithContext", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/valid-error-code-with-context", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp httpbase.R
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "REQ-1", resp.Msg)
		assert.NotNil(t, resp.Context)
	})
}

func TestShouldSkip(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"Healthz route", "/healthz", true},
		{"CSG route", "/csg", true},
		{"HF route", "/hf", true},
		{"CSG subroute", "/csg/api/v1", true},
		{"HF subroute", "/hf/models", true},
		{"Other route", "/api/v1/users", false},
		{"Root route", "/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &gin.Context{}
			req, _ := http.NewRequest("GET", tt.path, nil)
			c.Request = req
			got := shouldSkip(c)
			assert.Equal(t, tt.want, got)
		})
	}
}
