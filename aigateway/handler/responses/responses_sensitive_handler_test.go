package responses

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/rpc"
)

func TestHandleResponsesSensitiveResponseStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/", nil)

	HandleSensitiveResponse(c, true, &rpc.CheckResult{IsSensitive: true, Reason: "toxic"})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	body := rec.Body.String()
	require.Contains(t, body, "event: response.completed\n")
	require.Contains(t, body, "data: [DONE]\n\n")
	require.Contains(t, body, BlockedMessage)

	// The payload must be valid ResponsesResponse JSON in the data line.
	start := strings.Index(body, "data: ")
	require.NotEqual(t, -1, start)
	end := strings.Index(body[start:], "\n")
	require.NotEqual(t, -1, end)
	dataLine := body[start+len("data: ") : start+end]
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(dataLine), &resp))
	require.Equal(t, "response", resp["object"])
	require.Equal(t, "completed", resp["status"])
}

func TestHandleResponsesSensitiveResponseNonStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/", nil)

	HandleSensitiveResponse(c, false, &rpc.CheckResult{IsSensitive: true, Reason: "toxic"})

	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "response", resp["object"])
	require.Equal(t, "completed", resp["status"])
	output := resp["output"].([]any)
	require.Len(t, output, 1)
	message := output[0].(map[string]any)
	require.Equal(t, "message", message["type"])
	content := message["content"].([]any)
	require.Len(t, content, 1)
	part := content[0].(map[string]any)
	require.Equal(t, "output_text", part["type"])
	require.Equal(t, BlockedMessage, part["text"])
}
