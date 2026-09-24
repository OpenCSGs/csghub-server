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

// The 0.1.1 contract puts the detailed ranking at the top level of the
// response.
func TestRank_TopLevelRankingShape(t *testing.T) {
	var gotPath string
	var gotBody RankRequest
	var rawBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var err error
		rawBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(rawBody, &gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ranking":[
				{"rank":1,"model":"deepseek-v4-flash-0731","provider_model":"deepseek-v4-flash","provider_base_url":"https://upstream/v1","score":0.9},
				{"rank":2,"model":"glm-5.1","provider_model":"glm-5.1","provider_base_url":"https://upstream/v1","score":0.8}
			],
			"index_version":"live-abc",
			"policy_version":"v2/1.0",
			"elapsed_ms":12.5
		}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, time.Second)
	require.NoError(t, err)

	resp, err := client.Rank(context.Background(), &RankRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Tools:    []json.RawMessage{json.RawMessage(`{"type":"function"}`)},
	})
	require.NoError(t, err)

	require.Equal(t, rankPath, gotPath)
	require.Len(t, gotBody.Messages, 1)
	require.Equal(t, "user", gotBody.Messages[0].Role)
	require.Len(t, gotBody.Tools, 1)
	require.NotContains(t, string(rawBody), "debug", "the diagnostic switch must not be part of the production request")

	require.Len(t, resp.Ranking, 2)
	require.Equal(t, "deepseek-v4-flash", resp.Ranking[0].ProviderModel)
	require.Equal(t, "deepseek-v4-flash-0731", resp.Ranking[0].Model)
	require.Equal(t, "live-abc", resp.IndexVersion)
	require.Equal(t, "v2/1.0", resp.PolicyVersion)
	require.InDelta(t, 12.5, resp.ElapsedMS, 1e-9)
}

// Newer deployments return an ordered model_list and move the detailed
// ranking under "detail", which is only present because Rank asks for it.
func TestRank_DetailShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"model_list":["deepseek-v4-flash-0731","glm-5.1"],
			"detail":{
				"ranking":[
					{"rank":1,"model":"deepseek-v4-flash-0731","provider_model":"deepseek-v4-flash","score":0.9},
					{"rank":2,"model":"glm-5.1","provider_model":"glm-5.1","score":0.8}
				],
				"index_version":"live-xyz",
				"policy_version":"v2/1.0",
				"elapsed_ms":7.5
			}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, time.Second)
	require.NoError(t, err)

	resp, err := client.Rank(context.Background(), &RankRequest{Messages: []Message{{Role: "user", Content: "hello"}}})
	require.NoError(t, err)
	require.Len(t, resp.Ranking, 2)
	require.Equal(t, "deepseek-v4-flash", resp.Ranking[0].ProviderModel)
	require.Equal(t, "live-xyz", resp.IndexVersion)
}

// A deployment that withholds the detail leaves only the ordered
// benchmark identifiers, which still carry the ranking order.
func TestRank_SlimModelListShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"model_list":["deepseek-v4-flash-0731","glm-5.1","qwen3.8-flash"]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, time.Second)
	require.NoError(t, err)

	resp, err := client.Rank(context.Background(), &RankRequest{Messages: []Message{{Role: "user", Content: "hello"}}})
	require.NoError(t, err)
	require.Len(t, resp.Ranking, 3)
	require.Equal(t, 1, resp.Ranking[0].Rank)
	require.Equal(t, "deepseek-v4-flash-0731", resp.Ranking[0].Model)
	require.Empty(t, resp.Ranking[0].ProviderModel, "the upstream model name is simply not reported here")
	require.Equal(t, 3, resp.Ranking[2].Rank)
	require.Empty(t, resp.IndexVersion)
}

func TestRank_Errors(t *testing.T) {
	t.Run("no messages", func(t *testing.T) {
		client, err := NewClient("http://router.test", time.Second)
		require.NoError(t, err)
		_, err = client.Rank(context.Background(), &RankRequest{})
		require.ErrorContains(t, err, "at least one message")
	})

	t.Run("non 200 carries the body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"detail":"messages_require_current_user_request"}`))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, time.Second)
		require.NoError(t, err)
		_, err = client.Rank(context.Background(), &RankRequest{Messages: []Message{{Role: "system", Content: "x"}}})
		require.ErrorContains(t, err, "422")
		require.ErrorContains(t, err, "messages_require_current_user_request")
	})

	t.Run("empty ranking is rejected", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"ranking":[],"model_list":[]}`))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, time.Second)
		require.NoError(t, err)
		_, err = client.Rank(context.Background(), &RankRequest{Messages: []Message{{Role: "user", Content: "x"}}})
		require.ErrorContains(t, err, "empty ranking")
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
			Messages: []Message{{Role: "user", Content: strings.Repeat("a", maxRankRequestBytes+1)}},
		})
		require.ErrorContains(t, err, "over the")
		require.False(t, called)
	})
}

func TestReady(t *testing.T) {
	t.Run("ready, carrying the index version", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, readyPath, r.URL.Path)
			_, _ = w.Write([]byte(`{"ready":true,"index_version":"live-abc","training_requests":4214}`))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, time.Second)
		require.NoError(t, err)
		readiness, err := client.Ready(context.Background())
		require.NoError(t, err)
		require.Equal(t, "live-abc", readiness.IndexVersion)
		require.Equal(t, 4214, readiness.TrainingRequests)
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
