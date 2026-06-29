package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonutils "opencsg.com/csghub-server/common/utils/common"
)

// captureLog is a helper that installs a slog JSON handler writing to a buffer
// and returns a function to read all emitted log records.
func captureLog(t *testing.T) func() []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(orig) })

	return func() []map[string]any {
		var records []map[string]any
		for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
			if len(line) == 0 {
				continue
			}
			var m map[string]any
			err := json.Unmarshal(line, &m)
			require.NoError(t, err)
			records = append(records, m)
		}
		return records
	}
}

func TestLog_NormalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	readLogs := captureLog(t)

	r := gin.New()
	r.Use(Log())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	logs := readLogs()
	require.Len(t, logs, 1, "should emit exactly one log record")
	assert.Equal(t, float64(200), logs[0]["status"])
	assert.Equal(t, "/test", logs[0]["url"])
}

func TestLog_ClientCancelBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	readLogs := captureLog(t)

	r := gin.New()
	r.Use(Log())
	r.GET("/test", func(c *gin.Context) {
		// Handler still runs even though client already canceled
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Simulate client that cancels the request before the handler finishes
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logs := readLogs()
	require.Len(t, logs, 1, "must emit a log even when client canceled")
	assert.Equal(t, float64(commonutils.StatusClientClosedRequest), logs[0]["status"], "status should be 499 for client cancel")
	assert.Equal(t, "/test", logs[0]["url"])
}

func TestLog_ClientCancelDuringSlowHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	readLogs := captureLog(t)

	r := gin.New()
	r.Use(Log())
	r.GET("/slow", func(c *gin.Context) {
		// Simulate slow handler; client cancels mid-way
		select {
		case <-c.Request.Context().Done():
			// Client disconnected, return without writing response
			return
		case <-time.After(5 * time.Second):
			c.JSON(http.StatusOK, gin.H{"ok": true})
		}
	})

	// Use WithCancel (not WithTimeout) so Err() returns context.Canceled
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	req := httptest.NewRequest(http.MethodGet, "/slow", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logs := readLogs()
	require.Len(t, logs, 1, "must emit a log when client cancels during slow handler")
	assert.Equal(t, float64(commonutils.StatusClientClosedRequest), logs[0]["status"], "status should be 499")
}

func TestLog_HandlerPanicWithRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	readLogs := captureLog(t)

	r := gin.New()
	// Recovery must be registered BEFORE Log (same order as SetInfraMiddleware)
	r.Use(Recovery())
	r.Use(Log())
	r.GET("/panic", func(c *gin.Context) {
		panic("broken pipe!")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	logs := readLogs()
	// Should have at least 2 log records: one from Log middleware, one from Recovery
	require.Len(t, logs, 2, "should emit both Log and Recovery records")

	// Find the "http request" log
	var httpLog map[string]any
	for _, l := range logs {
		if l["msg"] == "http request" {
			httpLog = l
			break
		}
	}
	require.NotNil(t, httpLog, "Log middleware must emit 'http request' even when handler panics")
	assert.Equal(t, float64(500), httpLog["status"], "panic should log status 500")
	assert.Equal(t, "/panic", httpLog["url"])
}

func TestLog_HealthzSkipped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	readLogs := captureLog(t)

	r := gin.New()
	r.Use(Log())
	r.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	logs := readLogs()
	assert.Len(t, logs, 0, "healthz requests should not produce any log")
}

func TestLog_ServerErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	readLogs := captureLog(t)

	r := gin.New()
	r.Use(Log())
	r.GET("/error", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail"})
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	logs := readLogs()
	require.Len(t, logs, 1)
	assert.Equal(t, float64(500), logs[0]["status"])
}
func TestLog_BadRequestStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	readLogs := captureLog(t)

	r := gin.New()
	r.Use(Log())
	r.POST("/chat/completions", func(c *gin.Context) {
		// Simulate the same pattern as aigateway handler:
		// BindJSON fails → return 400
		var body struct {
			Stream bool `json:"stream"`
		}
		if err := c.BindJSON(&body); err != nil {
			slog.ErrorContext(c.Request.Context(), "invalid chat completion request body", slog.Any("error", err))
			c.String(http.StatusBadRequest, "invalid chat completion request body:"+err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Send invalid JSON: stream should be bool but we pass a string
	req := httptest.NewRequest(http.MethodPost, "/chat/completions",
		bytes.NewBufferString(`{"stream": "yes"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	logs := readLogs()
	// Should have at least 1 log: the "http request" from Log middleware
	// (the handler's slog.ErrorContext is a separate log line)
	var httpLog map[string]any
	for _, l := range logs {
		if l["msg"] == "http request" {
			httpLog = l
			break
		}
	}
	require.NotNil(t, httpLog, "Log middleware must emit 'http request' for 400 responses")
	assert.Equal(t, float64(400), httpLog["status"], "status should be 400")
	assert.Equal(t, "/chat/completions", httpLog["url"])
	assert.Equal(t, "POST", httpLog["method"])
}
