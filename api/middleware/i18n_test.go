package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/i18n"
	"opencsg.com/csghub-server/common/types"
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

	// Test case 8: 206 Partial Content response (range request)
	router.GET("/range", func(c *gin.Context) {
		body := []byte{0xe0, 0x8a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		c.Header("Content-Range", "bytes 0-7/1503300328")
		c.Header("Content-Length", "8")
		c.Data(http.StatusPartialContent, "application/octet-stream", body)
	})

	router.GET("/mirror-sync-cancelled", func(c *gin.Context) {
		httpbase.ConflictError(c, errorx.MirrorRepoSyncCanceled(
			errors.New("repository synchronization was canceled"),
			errorx.Ctx().Set("failure_reason", types.MirrorSyncFailureCanceled),
		))
	})
	router.GET("/mirror-sync-failed", func(c *gin.Context) {
		httpbase.ConflictError(c, errorx.MirrorRepoSyncFailed(
			errors.New("repository synchronization failed"),
			errorx.Ctx().Set("failure_reason", types.MirrorSyncFailureRepoSyncFailed),
		))
	})
	router.GET("/source-namespace-mapping-exists", func(c *gin.Context) {
		httpbase.ConflictError(c, errorx.SourceNamespaceMappingExists(
			errors.New("source namespace mapping exists"),
			errorx.Ctx().Set("source_namespace", "SourceTeam"),
		))
	})
	router.GET("/source-namespace-mapping-not-found", func(c *gin.Context) {
		httpbase.NotFoundError(c, errorx.SourceNamespaceMappingNotFound(
			errors.New("source namespace mapping does not exist"),
			errorx.Ctx().Set("source_namespace", "SourceTeam"),
		))
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

	t.Run("PartialContentResponse", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/range", nil)
		req.Header.Set("Range", "bytes=0-7")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusPartialContent, w.Code)
		// Body must be delivered intact — not silently discarded by the middleware
		expected := []byte{0xe0, 0x8a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		assert.Equal(t, expected, w.Body.Bytes())
	})

	t.Run("LocalizedCancelledMirrorSync", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/mirror-sync-cancelled", nil)
		req.Header.Set("Accept-Language", "zh-CN")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		var resp httpbase.R
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "MIRROR-ERR-4: 仓库同步已取消。", resp.Msg)
	})

	t.Run("LocalizedFailedMirrorSync", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/mirror-sync-failed", nil)
		req.Header.Set("Accept-Language", "zh-CN")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		var resp httpbase.R
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "MIRROR-ERR-2: 仓库同步失败。", resp.Msg)
	})

	t.Run("LocalizedSourceNamespaceMappingExists", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/source-namespace-mapping-exists", nil)
		req.Header.Set("Accept-Language", "zh-CN")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		var resp httpbase.R
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "MIRROR-ERR-6: 源命名空间已存在映射关系。", resp.Msg)
	})

	t.Run("LocalizedSourceNamespaceMappingNotFound", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/source-namespace-mapping-not-found", nil)
		req.Header.Set("Accept-Language", "zh-CN")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		var resp httpbase.R
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "MIRROR-ERR-7: 源命名空间映射关系不存在。", resp.Msg)
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
		{"Storage gateway route", "/api/v1/storage/bucket/key", true},
		{"Storage gateway root", "/api/v1/storage", true},
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
