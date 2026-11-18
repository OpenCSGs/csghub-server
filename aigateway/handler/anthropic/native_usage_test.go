package anthropic

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestNativeUsage_PromptTokensIncludeCache_NonStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	w := newNativeResponseWriter(c.Writer, false, "pub-model", nil)
	body := `{"id":"msg_1","type":"message","role":"assistant","model":"up","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":100,"cache_creation_input_tokens":20}}`
	_, err := w.Write([]byte(body))
	assert.NoError(t, err)
	assert.NoError(t, w.Finalize())

	u := w.Usage()
	assert.Equal(t, int64(130), u.PromptTokens)
	assert.Equal(t, int64(100), u.CachedPromptTokens)
	assert.Equal(t, int64(20), u.CacheCreationPromptTokens)
	assert.Equal(t, int64(135), u.TotalTokens)
	assert.LessOrEqual(t, u.CachedPromptTokens, u.PromptTokens)
}

func TestNativeUsage_PromptTokensIncludeCache_Stream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	w := newNativeResponseWriter(c.Writer, true, "pub-model", nil)
	w.extractUsageFromNativeSSE("message_start", []byte(`{"message":{"id":"msg_1","usage":{"input_tokens":10,"cache_read_input_tokens":100,"cache_creation_input_tokens":20}}}`))
	w.extractUsageFromNativeSSE("message_delta", []byte(`{"delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`))

	u := w.Usage()
	assert.Equal(t, int64(130), u.PromptTokens)
	assert.Equal(t, int64(135), u.TotalTokens)
	assert.LessOrEqual(t, u.CachedPromptTokens, u.PromptTokens)
}
