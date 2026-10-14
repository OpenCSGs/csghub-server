package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestChatStreamViaChatCompletions_SDKChunks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte(`data: {"id":"1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"he"}}]}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte(`data: {"id":"1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"llo"},"finish_reason":"stop"}]}` + "\n\n"))
		flusher.Flush()
	}))
	defer srv.Close()

	c := NewClient()
	ch, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatChat)).ChatStream(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
			Stream:   true,
		})
	require.NoError(t, err)
	assert.Equal(t, []string{"he", "llo"}, collectStream(t, ch))
}

func TestChatWithUpstream_UndeclaredPassthroughStream(t *testing.T) {
	// An upstream without any protocol declaration keeps the legacy raw
	// line passthrough to the URL as-is.
	chunk := `{"id":"1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"raw"}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/custom/endpoint", r.URL.Path)
		_, _ = w.Write([]byte(chunk + "\n"))
	}))
	defer srv.Close()

	cfg := &database.LLMConfig{
		ModelName: "config-model",
		Upstreams: []database.Upstream{{
			URL:     srv.URL + "/custom/endpoint",
			Enabled: true,
		}},
	}
	c := NewClient()
	ch, err := c.WithLLMConfig(cfg).ChatStream(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
			Stream:   true,
		})
	require.NoError(t, err)

	select {
	case line, ok := <-ch:
		require.True(t, ok)
		assert.Equal(t, chunk, line)
	case <-time.After(2 * time.Second):
		t.Fatal("no chunk received from passthrough stream")
	}
}
