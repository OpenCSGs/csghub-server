package semanticrouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		err   bool
	}{
		{name: "plain", input: "https://router.test:1032", want: "https://router.test:1032"},
		{name: "trailing slash", input: "https://router.test:1032/", want: "https://router.test:1032"},
		{name: "v1 suffix", input: "https://router.test:1032/v1", want: "https://router.test:1032"},
		{name: "surrounding space", input: "  http://router.test  ", want: "http://router.test"},
		{name: "empty", input: "   ", err: true},
		{name: "no scheme", input: "router.test:1032", err: true},
		{name: "unsupported scheme", input: "ftp://router.test", err: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeBaseURL(tc.input)
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// The agent-turn-input/1 contract splits the conversation into a system
// prompt, a history and a current input, and echoes back the session and
// turn identity it assigned in the CSG-Router-* response headers.
func TestRank_NewSessionShape(t *testing.T) {
	var gotPath string
	var gotRequestBody RankRequest
	var rawBody []byte
	var gotSessionHeader, gotTurnHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSessionHeader = r.Header.Get(headerSessionID)
		gotTurnHeader = r.Header.Get(headerTurnID)
		var err error
		rawBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(rawBody, &gotRequestBody))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(headerSessionID, "ses_0123456789abcdef0123456789abcdef")
		w.Header().Set(headerTurnID, "turn_0123456789abcdef0123456789abcdef")
		_, _ = w.Write([]byte(`{"model_list":["qwen3.8-flash","glm-5.3-flash","deepseek-v4-flash"]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, time.Second)
	require.NoError(t, err)

	resp, err := client.Rank(context.Background(), &RankRequest{
		SystemPrompt: "You are a coding agent.",
		Tools:        []json.RawMessage{json.RawMessage(`{"type":"function"}`)},
		History:      []Message{{Role: "user", Content: "what does this do?"}, {Role: "assistant", Content: "it checks"}},
		CurrentInput: []Message{{Role: "user", Content: "check the failing test"}},
	})
	require.NoError(t, err)

	require.Equal(t, rankPath, gotPath)
	require.Equal(t, "You are a coding agent.", gotRequestBody.SystemPrompt)
	require.Len(t, gotRequestBody.Tools, 1)
	require.Len(t, gotRequestBody.History, 2)
	require.Len(t, gotRequestBody.CurrentInput, 1)
	require.Equal(t, "user", gotRequestBody.CurrentInput[0].Role)
	require.Empty(t, gotSessionHeader, "a new session omits the session header")
	require.Empty(t, gotTurnHeader, "a new turn omits the turn header")
	require.NotContains(t, string(rawBody), "session_id", "routing identity is header-only")
	require.NotContains(t, string(rawBody), "turn_id", "routing identity is header-only")
	require.NotContains(t, string(rawBody), "debug", "the diagnostic switch must not be part of the production request")

	require.Equal(t, "ses_0123456789abcdef0123456789abcdef", resp.SessionID)
	require.Equal(t, "turn_0123456789abcdef0123456789abcdef", resp.TurnID)
	require.Equal(t, []string{"qwen3.8-flash", "glm-5.3-flash", "deepseek-v4-flash"}, resp.ModelList)
}

// A repeated call within the same turn carries both IDs in the request
// headers so the service can return the ranking it already saved for that
// turn, and the identity is echoed back through the response headers.
func TestRank_SameTurnRepeatCarriesIDs(t *testing.T) {
	var rawBody []byte
	var gotSessionHeader, gotTurnHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		rawBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		gotSessionHeader = r.Header.Get(headerSessionID)
		gotTurnHeader = r.Header.Get(headerTurnID)
		w.Header().Set(headerSessionID, "ses_0123456789abcdef0123456789abcdef")
		w.Header().Set(headerTurnID, "turn_0123456789abcdef0123456789abcdef")
		_, _ = w.Write([]byte(`{"model_list":["glm-5.3-flash"]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, time.Second)
	require.NoError(t, err)

	resp, err := client.Rank(context.Background(), &RankRequest{
		SystemPrompt: "You are a coding agent.",
		CurrentInput: []Message{{Role: "user", Content: "again"}},
		SessionID:    "ses_0123456789abcdef0123456789abcdef",
		TurnID:       "turn_0123456789abcdef0123456789abcdef",
	})
	require.NoError(t, err)
	require.Equal(t, "ses_0123456789abcdef0123456789abcdef", gotSessionHeader)
	require.Equal(t, "turn_0123456789abcdef0123456789abcdef", gotTurnHeader)
	require.NotContains(t, string(rawBody), "session_id", "routing identity is header-only")
	require.NotContains(t, string(rawBody), "turn_id", "routing identity is header-only")
	require.Equal(t, "ses_0123456789abcdef0123456789abcdef", resp.SessionID)
	require.Equal(t, "turn_0123456789abcdef0123456789abcdef", resp.TurnID)
}

func TestRank_Errors(t *testing.T) {
	t.Run("no current input", func(t *testing.T) {
		client, err := NewClient("http://router.test", time.Second)
		require.NoError(t, err)
		_, err = client.Rank(context.Background(), &RankRequest{SystemPrompt: "You are a coding agent."})
		require.ErrorContains(t, err, "at least one current input message")
	})

	t.Run("non 200 carries the body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"detail":"messages_require_current_user_request"}`))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, time.Second)
		require.NoError(t, err)
		_, err = client.Rank(context.Background(), &RankRequest{
			SystemPrompt: "You are a coding agent.",
			CurrentInput: []Message{{Role: "system", Content: "x"}},
		})
		require.ErrorContains(t, err, "422")
		require.ErrorContains(t, err, "messages_require_current_user_request")
	})

	t.Run("empty model list is rejected", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"model_list":[]}`))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, time.Second)
		require.NoError(t, err)
		_, err = client.Rank(context.Background(), &RankRequest{
			SystemPrompt: "You are a coding agent.",
			CurrentInput: []Message{{Role: "user", Content: "x"}},
		})
		require.ErrorContains(t, err, "empty model list")
	})

	t.Run("oversized body is not sent", func(t *testing.T) {
		var called bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		}))
		defer server.Close()

		client, err := NewClient(server.URL, time.Second)
		require.NoError(t, err)
		_, err = client.Rank(context.Background(), &RankRequest{
			SystemPrompt: "You are a coding agent.",
			CurrentInput: []Message{{Role: "user", Content: strings.Repeat("a", maxRankRequestBytes+1)}},
		})
		require.ErrorContains(t, err, "over the")
		require.False(t, called)
	})
}

func TestReady(t *testing.T) {
	t.Run("ready, carrying the index version", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, readyPath, r.URL.Path)
			_, _ = w.Write([]byte(`{"ready":true,"index_version":"live-abc","training_turns":4214}`))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, time.Second)
		require.NoError(t, err)
		readiness, err := client.Ready(context.Background())
		require.NoError(t, err)
		require.Equal(t, "live-abc", readiness.IndexVersion)
		require.Equal(t, 4214, readiness.TrainingTurns)
	})

	t.Run("answering but not ready", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"ready":false,"index_version":""}`))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, time.Second)
		require.NoError(t, err)
		_, err = client.Ready(context.Background())
		require.ErrorContains(t, err, "not ready")
	})

	t.Run("unhealthy status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, time.Second)
		require.NoError(t, err)
		_, err = client.Ready(context.Background())
		require.ErrorContains(t, err, "503")
	})

	t.Run("unreachable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := server.URL
		server.Close()

		client, err := NewClient(url, 200*time.Millisecond)
		require.NoError(t, err)
		_, err = client.Ready(context.Background())
		require.Error(t, err)
	})
}
