package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/common/utils/trace"
)

func TestRequest(t *testing.T) {
	// 1. Set up the Gin router in test mode
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Request())

	// 2. Define a test handler to check the context and headers
	router.GET("/test", func(c *gin.Context) {
		// Assert that clientIP is set in Gin's context
		clientIP, exists := c.Get("clientIP")
		assert.True(t, exists, "clientIP should exist in gin context")
		assert.Equal(t, "192.0.2.1", clientIP, "clientIP in gin context should match")

		// Assert that clientIP is set in the request's context
		reqCtxClientIP := c.Request.Context().Value("clientIP")
		assert.NotNil(t, reqCtxClientIP, "clientIP should exist in request context")
		assert.Equal(t, "192.0.2.1", reqCtxClientIP.(string), "clientIP in request context should match")

		c.Status(http.StatusOK)
	})

	// 3. Create a new HTTP request
	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	assert.NoError(t, err)
	// Simulate a client IP
	req.RemoteAddr = "192.0.2.1:12345"

	// 4. Create a response recorder to capture the response
	w := httptest.NewRecorder()

	// 5. Serve the HTTP request
	router.ServeHTTP(w, req)

	// 6. Assert the response status and headers
	assert.Equal(t, http.StatusOK, w.Code, "HTTP status should be 200 OK")
	// Assert that the trace ID header is set in the response
	traceID := w.Header().Get(trace.HeaderRequestID)
	assert.NotEmpty(t, traceID, "X-Request-ID header should be set")
}
