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

func TestNativeUsage_MessageDeltaCumulativeUsage_Stream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	w := newNativeResponseWriter(c.Writer, true, "pub-model", nil)
	// Upstream sends zero usage in message_start and the real totals in message_delta.
	w.extractUsageFromNativeSSE("message_start", []byte(`{"message":{"id":"msg_1","usage":{"input_tokens":0,"output_tokens":1}}}`))
	w.extractUsageFromNativeSSE("message_delta", []byte(`{"delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":10,"cache_read_input_tokens":100,"cache_creation_input_tokens":20,"output_tokens":5}}`))

	u := w.Usage()
	assert.Equal(t, int64(130), u.PromptTokens)
	assert.Equal(t, int64(100), u.CachedPromptTokens)
	assert.Equal(t, int64(20), u.CacheCreationPromptTokens)
	assert.Equal(t, int64(135), u.TotalTokens)
}

func TestNativeUsage_MessageDeltaWithoutInputKeepsMessageStart_Stream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	w := newNativeResponseWriter(c.Writer, true, "pub-model", nil)
	w.extractUsageFromNativeSSE("message_start", []byte(`{"message":{"id":"msg_1","usage":{"input_tokens":10,"cache_read_input_tokens":100}}}`))
	w.extractUsageFromNativeSSE("message_delta", []byte(`{"delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`))

	assert.Equal(t, int64(110), w.Usage().PromptTokens)
}

func TestNativeUsage_MessageDeltaZeroInputKeepsMessageStart_Stream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	w := newNativeResponseWriter(c.Writer, true, "pub-model", nil)
	w.extractUsageFromNativeSSE("message_start", []byte(`{"message":{"id":"msg_1","usage":{"input_tokens":10,"cache_read_input_tokens":100}}}`))
	w.extractUsageFromNativeSSE("message_delta", []byte(`{"delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0,"output_tokens":5}}`))

	assert.Equal(t, int64(110), w.Usage().PromptTokens)
}
