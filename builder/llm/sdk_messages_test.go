package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestChatViaMessages_NonStreaming(t *testing.T) {
	var gotPath string
	var gotAPIKey string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("X-Api-Key")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer srv.Close()

	c := NewClient()
	cfg := upstreamConfigWithURL(srv.URL, types.ApiFormatMessages)
	cfg.Upstreams[0].AuthHeader = `{"x-api-key":"sk-ant"}`
	text, err := c.WithLLMConfig(cfg).Chat(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{
				{Role: systemRole, Content: "system prompt"},
				{Role: "user", Content: "user text"},
			},
			Stream:      false,
			Temperature: 0.2,
		})
	require.NoError(t, err)
	assert.Equal(t, "ok", text)
	assert.Equal(t, "/v1/messages", gotPath)
	assert.Equal(t, "sk-ant", gotAPIKey)
	assert.Equal(t, "upstream-model", gotBody["model"])
	assert.InEpsilon(t, float64(defaultAnthropicMaxTokens), gotBody["max_tokens"], 0.0001)
	system, ok := gotBody["system"].([]any)
	require.True(t, ok)
	require.Len(t, system, 1)
	assert.Equal(t, "system prompt", system[0].(map[string]any)["text"])
	messages, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1)
	assert.Equal(t, "user", messages[0].(map[string]any)["role"])
	assert.InEpsilon(t, 0.2, gotBody["temperature"], 0.0001)
}

func TestChatViaMessages_ReqMaxTokensWins(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hi"}]}`))
	}))
	defer srv.Close()

	c := NewClient()
	_, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatMessages)).Chat(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages:  []types.LLMMessage{{Role: "user", Content: "user text"}},
			MaxTokens: 100,
		})
	require.NoError(t, err)
	assert.InEpsilon(t, float64(100), gotBody["max_tokens"], 0.0001)
}

func TestChatViaMessages_HostOverride(t *testing.T) {
	var gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	c := NewClient()
	cfg := upstreamConfigWithURL(srv.URL, types.ApiFormatMessages)
	cfg.Upstreams[0].Metadata = &types.UpstreamMetadata{
		InternalModelInfo: &types.InternalModelInfo{Host: "custom-host.example.com"},
	}
	_, err := c.WithLLMConfig(cfg).Chat(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
		})
	require.NoError(t, err)
	assert.Equal(t, "custom-host.example.com", gotHost)
}

func TestChatViaMessages_QueryBearingFullURL(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	// A full-URL upstream whose endpoint carries a query string (e.g.
	// Azure-style api-version): the /v1/messages tail must be stripped
	// from the SDK base path while the query is preserved, so the final
	// request is /v1/messages?api-version=... exactly once.
	c := NewClient()
	cfg := &database.LLMConfig{
		ModelName: "config-model",
		Upstreams: []database.Upstream{{
			URL:                srv.URL + "/v1/messages?api-version=2024-10-01",
			IsFullURL:          true,
			Enabled:            true,
			ModelName:          "upstream-model",
			AuthHeader:         `{"x-api-key":"sk-ant"}`,
			SupportedEndpoints: map[types.ApiFormat]string{types.ApiFormatMessages: ""},
		}},
	}
	_, err := c.WithLLMConfig(cfg).Chat(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
		})
	require.NoError(t, err)
	assert.Equal(t, "/v1/messages", gotPath)
	assert.Equal(t, "api-version=2024-10-01", gotQuery)
}

func TestChatViaMessages_EmptyTextError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":""}]}`))
	}))
	defer srv.Close()

	c := NewClient()
	_, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatMessages)).Chat(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
		})
	require.ErrorIs(t, err, errInvalidSummary)
}

func TestChatStreamViaMessages_TextDeltaEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte("event: message_start\n" +
			`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("event: content_block_delta\n" +
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"wor"}}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("event: content_block_delta\n" +
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ld"}}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("event: message_stop\n" + `data: {"type":"message_stop"}` + "\n\n"))
		flusher.Flush()
	}))
	defer srv.Close()

	c := NewClient()
	ch, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatMessages)).ChatStream(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
			Stream:   true,
		})
	require.NoError(t, err)
	assert.Equal(t, []string{"wor", "ld"}, collectStream(t, ch))
}

func TestChatStreamViaMessages_CancelMidStreamClosesChannel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for i := 0; i < 20; i++ {
			_, err := fmt.Fprintf(w, "event: content_block_delta\n"+
				`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"chunk-%d"}}`+"\n\n", i)
			if err != nil {
				return
			}
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := NewClient()
	ch, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatMessages)).ChatStream(
		ctx, "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
			Stream:   true,
		})
	require.NoError(t, err)

	<-ch
	cancel()
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("stream channel was not closed after context cancellation")
	}
}

func TestChatStreamViaMessages_RequestErrorReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
	}))
	defer srv.Close()

	c := NewClient()
	ch, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatMessages)).ChatStream(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
			Stream:   true,
		})
	// Request-level upstream errors (e.g. 401) must be returned
	// synchronously, not swallowed into an empty stream.
	require.Error(t, err)
	assert.Nil(t, ch)
}

func TestChatViaMessages_EnvCredentialNotLeaked(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "ambient-env-key")
	var gotAPIKey, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-Api-Key")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	// Upstream without any auth config: the SDK env fallback credential
	// must not reach the upstream.
	cfg := upstreamConfigWithURL(srv.URL, types.ApiFormatMessages)
	cfg.Upstreams[0].AuthHeader = ""
	c := NewClient()
	_, err := c.WithLLMConfig(cfg).Chat(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
		})
	require.NoError(t, err)
	assert.Empty(t, gotAPIKey)
	assert.Empty(t, gotAuth)
}
