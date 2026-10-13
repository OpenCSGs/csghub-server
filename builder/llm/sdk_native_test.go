package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// collectStream drains a stream channel and returns the text deltas parsed
// from the emitted openai chunk JSON lines.
func collectStream(t *testing.T, ch <-chan string) []string {
	t.Helper()
	var deltas []string
	for line := range ch {
		var resp types.LLMResponse
		require.NoError(t, json.Unmarshal([]byte(line), &resp))
		require.Len(t, resp.Choices, 1)
		if resp.Choices[0].Delta.Content != "" {
			deltas = append(deltas, resp.Choices[0].Delta.Content)
		}
	}
	return deltas
}

func upstreamConfigWithURL(url string, format types.ApiFormat) *database.LLMConfig {
	return &database.LLMConfig{
		ModelName: "config-model",
		Upstreams: []database.Upstream{{
			URL:                url,
			Enabled:            true,
			ModelName:          "upstream-model",
			AuthHeader:         `{"Authorization":"Bearer sdk-token"}`,
			SupportedEndpoints: map[types.ApiFormat]string{format: ""},
		}},
	}
}

func TestChatWithUpstream_DeclaredChatViaSDK(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"native chat"}}]}`))
	}))
	defer srv.Close()

	c := NewClient()
	text, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatChat)).Chat(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{
				{Role: systemRole, Content: "system prompt"},
				{Role: "user", Content: "user text"},
			},
			Stream:      false,
			Temperature: 0.2,
		})
	require.NoError(t, err)
	assert.Equal(t, "native chat", text)
	// The SDK appends the chat completions resource path to the derived
	// base and keeps the openai chat wire format.
	assert.Equal(t, "/v1/chat/completions", gotPath)
	assert.Equal(t, "upstream-model", gotBody["model"])
	messages, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)
}

func TestChatWithUpstream_ModelNotCrossContaminated(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	// WithLLMConfig must not mutate the shared config: a healthy
	// systemone-only upstream selected by the legacy derived fields (or
	// concurrently by another client) must not leak its model name into
	// the call that selects the chat upstream.
	cfg := &database.LLMConfig{
		ModelName: "config-model",
		Upstreams: []database.Upstream{
			{
				URL:                srv.URL + "/jev",
				Enabled:            true,
				ModelName:          "jev-model",
				SupportedEndpoints: map[types.ApiFormat]string{types.ApiFormatSystemOne: ""},
			},
			{
				URL:                srv.URL,
				Enabled:            true,
				SupportedEndpoints: map[types.ApiFormat]string{types.ApiFormatChat: ""},
			},
		},
	}
	c := NewClient()
	_, err := c.WithLLMConfig(cfg).Chat(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
		})
	require.NoError(t, err)
	assert.Equal(t, "config-model", gotBody["model"])
	assert.Equal(t, "config-model", cfg.ModelName, "the caller's config must not be mutated")
}
