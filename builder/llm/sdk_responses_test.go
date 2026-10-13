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

	"opencsg.com/csghub-server/common/types"
)

func TestChatViaResponses_NonStreaming(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"status":"completed"}`))
	}))
	defer srv.Close()

	c := NewClient()
	text, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatResponses)).Chat(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{
				{Role: systemRole, Content: "system prompt"},
				{Role: "user", Content: "user text"},
			},
			Stream:      false,
			Temperature: 0.2,
		})
	require.NoError(t, err)
	assert.Equal(t, "hello", text)
	assert.Equal(t, "/v1/responses", gotPath)
	assert.Equal(t, "Bearer sdk-token", gotAuth)
	assert.Equal(t, "upstream-model", gotBody["model"])
	assert.Equal(t, "system prompt", gotBody["instructions"])
	input, ok := gotBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	item, ok := input[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", item["role"])
	assert.Equal(t, "user text", item["content"])
	assert.InEpsilon(t, 0.2, gotBody["temperature"], 0.0001)
	assert.NotContains(t, gotBody, "max_output_tokens")
}

func TestChatViaResponses_MaxOutputTokens(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","output":[{"type":"message","content":[{"type":"output_text","text":"hi"}]}]}`))
	}))
	defer srv.Close()

	c := NewClient()
	_, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatResponses)).Chat(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages:  []types.LLMMessage{{Role: "user", Content: "user text"}},
			MaxTokens: 100,
		})
	require.NoError(t, err)
	assert.InEpsilon(t, float64(100), gotBody["max_output_tokens"], 0.0001)
}

func TestChatViaResponses_EmptyTextError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","output":[]}`))
	}))
	defer srv.Close()

	c := NewClient()
	_, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatResponses)).Chat(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
		})
	require.ErrorIs(t, err, errInvalidSummary)
}

func TestChatStreamViaResponses_TextDeltaEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte("event: response.output_text.delta\n" +
			`data: {"type":"response.output_text.delta","sequence_number":1,"item_id":"i","output_index":0,"content_index":0,"delta":"he"}` + "\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("event: response.output_text.delta\n" +
			`data: {"type":"response.output_text.delta","sequence_number":2,"item_id":"i","output_index":0,"content_index":0,"delta":"llo"}` + "\n\n"))
		flusher.Flush()
	}))
	defer srv.Close()

	c := NewClient()
	ch, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatResponses)).ChatStream(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
			Stream:   true,
		})
	require.NoError(t, err)
	assert.Equal(t, []string{"he", "llo"}, collectStream(t, ch))
}

func TestChatStreamViaResponses_MidStreamErrorEmitsErrorLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte("event: response.output_text.delta\n" +
			`data: {"type":"response.output_text.delta","sequence_number":1,"item_id":"i","output_index":0,"content_index":0,"delta":"he"}` + "\n\n"))
		flusher.Flush()
		// Abort the connection mid-stream: the consumer must see an error
		// line, not a clean end-of-stream.
		panic(http.ErrAbortHandler)
	}))
	defer srv.Close()

	c := NewClient()
	ch, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatResponses)).ChatStream(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
			Stream:   true,
		})
	require.NoError(t, err)

	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}
	require.NotEmpty(t, lines)
	var last map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &last))
	errBody, ok := last["error"].(map[string]any)
	require.True(t, ok, "mid-stream abort must end with an error line, got %v", lines[len(lines)-1])
	assert.NotEmpty(t, errBody["message"])
}

func TestChatStreamViaResponses_CancelMidStreamClosesChannel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for i := 0; i < 20; i++ {
			_, err := fmt.Fprintf(w, "event: response.output_text.delta\n"+
				`data: {"type":"response.output_text.delta","sequence_number":%d,"item_id":"i","output_index":0,"content_index":0,"delta":"chunk"}`+"\n\n", i)
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
	ch, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatResponses)).ChatStream(
		ctx, "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
			Stream:   true,
		})
	require.NoError(t, err)

	// Read one delta, then cancel and abandon the channel: the streaming
	// goroutine must abort on ctx cancellation and close the channel
	// instead of blocking forever.
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

func TestChatStreamViaResponses_RequestErrorReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key","type":"invalid_request_error"}}`))
	}))
	defer srv.Close()

	c := NewClient()
	ch, err := c.WithLLMConfig(upstreamConfigWithURL(srv.URL, types.ApiFormatResponses)).ChatStream(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
			Stream:   true,
		})
	// Request-level upstream errors (e.g. 401) must be returned
	// synchronously, not swallowed into an empty stream.
	require.Error(t, err)
	assert.Nil(t, ch)
}

func TestChatViaResponses_EnvCredentialNotLeaked(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "ambient-env-key")
	t.Setenv("OPENAI_ORG_ID", "ambient-org")
	t.Setenv("OPENAI_PROJECT_ID", "ambient-project")
	var gotAuth, gotOrg, gotProject string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotOrg = r.Header.Get("OpenAI-Organization")
		gotProject = r.Header.Get("OpenAI-Project")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`))
	}))
	defer srv.Close()

	// Upstream without any auth config: the SDK env fallback credentials
	// and platform identifiers must not reach the upstream.
	cfg := upstreamConfigWithURL(srv.URL, types.ApiFormatResponses)
	cfg.Upstreams[0].AuthHeader = ""
	c := NewClient()
	_, err := c.WithLLMConfig(cfg).Chat(
		context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "user text"}},
		})
	require.NoError(t, err)
	assert.Empty(t, gotAuth)
	assert.Empty(t, gotOrg)
	assert.Empty(t, gotProject)
}
