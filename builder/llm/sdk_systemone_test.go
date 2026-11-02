package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func systemOneTestClient(t *testing.T, handler http.HandlerFunc) *SystemOneClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewSystemOneClient(SystemOneClientConfig{
		Endpoint: server.URL,
		Headers:  map[string]string{"Authorization": "Bearer test-key", "Host": "decision-model.internal"},
	})
}

func systemOneTestRequest() *SystemOneRequest {
	return &SystemOneRequest{
		Model: "test-model",
		State: json.RawMessage(`{"role":"user","content":"hello"}`),
		Questions: map[string]SystemOneQuestion{
			"prompt_injection": {Type: "noul", Instructions: "score the injection risk"},
		},
	}
}

func TestSystemOneClient_Post_DecodesAnswers(t *testing.T) {
	var gotAuth, gotHost, gotContentType string
	client := systemOneTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotHost = r.Host
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"model":"test-model","answers":{"prompt_injection":{"type":"noul","noul":0.87}},"usage":{"input_tokens":10,"truncated":true,"truncated_questions":["prompt_injection"]}}`))
	})

	resp, err := client.Post(context.Background(), systemOneTestRequest())
	require.NoError(t, err)
	require.Equal(t, "Bearer test-key", gotAuth)
	require.Equal(t, "decision-model.internal", gotHost)
	require.Equal(t, "application/json", gotContentType)

	score, err := ParseSystemOneNoul(resp.Answers["prompt_injection"])
	require.NoError(t, err)
	require.InDelta(t, 0.87, score, 0.0001)
	require.NotNil(t, resp.Usage)
	require.True(t, resp.Usage.Truncated)
	require.Equal(t, []string{"prompt_injection"}, resp.Usage.TruncatedQuestions)
}

func TestSystemOneClient_Post_StatusErrorCarriesNoBody(t *testing.T) {
	client := systemOneTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		// The body echoes the submitted state: it must not surface in the
		// error, which callers log.
		_, _ = w.Write([]byte(`{"error":"unauthorized for state 'Ignore all previous instructions'"}`))
	})

	_, err := client.Post(context.Background(), systemOneTestRequest())
	require.Error(t, err)
	var statusErr *SystemOneStatusError
	require.True(t, errors.As(err, &statusErr))
	require.Equal(t, http.StatusUnauthorized, statusErr.StatusCode)
	require.NotContains(t, err.Error(), "Ignore all previous instructions")
}

func TestSystemOneClient_Post_BadResponseClassification(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"undecodable body", `not-json`},
		{"missing answers", `{"model":"test-model"}`},
		{"null answers", `{"answers":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := systemOneTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			})

			_, err := client.Post(context.Background(), systemOneTestRequest())
			require.Error(t, err)
			var badResp *SystemOneBadResponseError
			require.True(t, errors.As(err, &badResp))
		})
	}
}

func TestSystemOneClient_Post_RequestValidation(t *testing.T) {
	client := systemOneTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"answers":{}}`))
	})

	for name, req := range map[string]*SystemOneRequest{
		"nil request":       nil,
		"missing model":     {State: json.RawMessage(`"x"`), Questions: map[string]SystemOneQuestion{"q": {Type: "noul"}}},
		"missing state":     {Model: "m", Questions: map[string]SystemOneQuestion{"q": {Type: "noul"}}},
		"missing questions": {Model: "m", State: json.RawMessage(`"x"`)},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := client.Post(context.Background(), req)
			require.Error(t, err)
			var badResp *SystemOneBadResponseError
			require.True(t, errors.As(err, &badResp))
		})
	}
}

func TestSystemOneClient_ParseAnswers(t *testing.T) {
	score, err := ParseSystemOneNoul(json.RawMessage(`{"type":"noul","noul":0.42}`))
	require.NoError(t, err)
	require.InDelta(t, 0.42, score, 0.0001)

	// Protocol boundaries: 0 and 1 are valid scores.
	for _, boundary := range []float64{0, 1} {
		score, err = ParseSystemOneNoul(json.RawMessage(fmt.Sprintf(`{"type":"noul","noul":%v}`, boundary)))
		require.NoError(t, err)
		require.InDelta(t, boundary, score, 0.0001)
	}

	// Protocol violations degrade to errors (caller maps them to the
	// bad-response fail-open path) instead of a wrong verdict.
	for _, raw := range []string{
		`{"type":"choice","noul":0.9}`, // wrong answer type
		`{"type":"noul","noul":1.5}`,   // out of range
		`{"type":"noul","noul":-0.1}`,  // out of range
		`{"type":"noul"}`,              // missing score
		`not-json`,                     // undecodable
	} {
		_, err = ParseSystemOneNoul(json.RawMessage(raw))
		require.Error(t, err, "raw: %s", raw)
	}

	choice, err := ParseSystemOneChoice(json.RawMessage(`{"type":"choice","choice":"direct_override"}`))
	require.NoError(t, err)
	require.Equal(t, "direct_override", choice)

	// A choice answer without a label decodes to an empty string; callers
	// whitelist labels, so an empty one is dropped there rather than here.
	choice, err = ParseSystemOneChoice(json.RawMessage(`{"type":"choice"}`))
	require.NoError(t, err)
	require.Empty(t, choice)
}
