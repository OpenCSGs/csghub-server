package sample

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestDecisionsRegistryMatchesEndpointSuffixes(t *testing.T) {
	for _, endpoint := range []string{
		"https://api.openai.com/v1/decisions",
		"http://decision.internal:8000/v1/decisions",
		"https://api.example.com/decisions",
	} {
		provider, ok := NewDefaultRegistry().Find(endpoint)
		require.True(t, ok, "endpoint %s should match the decisions provider", endpoint)
		require.True(t, provider.Supports(endpoint))
	}

	// The decisions provider itself must reject non-decisions endpoints:
	// Find() returns the first matching provider, so a chat URL resolves to
	// the chat provider, never to decisions.
	decisions := newProtocolProvider(decisionsRoute, modelsL7Request(decisionsRoute), decisionsRequest, defaultSampleTimeout)
	for _, endpoint := range []string{
		"https://api.openai.com/v1/chat/completions",
		"https://api.example.com/decisionsextra",
		"https://api.example.com/decision",
	} {
		require.False(t, decisions.Supports(endpoint), "endpoint %s should not match the decisions provider", endpoint)
	}
}

func TestDecisionsInferenceSampleBody(t *testing.T) {
	endpoint := "https://api.openai.com/v1/decisions"
	provider, ok := NewDefaultRegistry().Find(endpoint)
	require.True(t, ok)

	request, err := sampleRequestBuilder(t, provider).Build(types.SampleKindInference, sampleInput(endpoint, "decision-model"))
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, request.Method)
	require.Equal(t, endpoint, request.Endpoint)
	require.Equal(t, "application/json", request.Headers.Get("Content-Type"))

	var body types.DecisionRequest
	require.NoError(t, json.Unmarshal(request.Body, &body))
	require.Equal(t, "decision-model", body.Model)
	require.Equal(t, "decision-model", body.ClientModel)
	require.Contains(t, string(body.Input), "hi", "the sample text is carried in the input")
	// Input and questions are opaque; assert only that they are present,
	// non-null JSON values.
	require.NotEmpty(t, body.Input)
	require.NotEqual(t, "null", string(body.Input))
	require.NotEmpty(t, body.Questions)
	require.NotEqual(t, "null", string(body.Questions))
	// The predicate question must use the official field name: OpenAI
	// rejects (or at best ignores) the non-standard "description".
	require.Contains(t, string(body.Questions), `"instructions"`)
	require.NotContains(t, string(body.Questions), `"description"`)
}

func TestDecisionsL7UsesModelsEndpoint(t *testing.T) {
	endpoint := "https://api.openai.com/v1/decisions"
	provider, ok := NewDefaultRegistry().Find(endpoint)
	require.True(t, ok)

	request, err := sampleRequestBuilder(t, provider).Build(types.SampleKindL7API, sampleInput(endpoint, "model"))
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, request.Method)
	require.Equal(t, "https://api.openai.com/v1/models", request.Endpoint)
}

func TestDecisionsL7FallbackToInference(t *testing.T) {
	// An upstream without a /models endpoint: the L7 probe gets a 404 and
	// must flag the inference fallback so the health checker probes with a
	// real (minimal) Decisions call instead.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/decisions" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"m","answers":[{"type":"predicate","name":"sample","value":true}],"usage":{"input_tokens":1,"output_tokens":1}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	endpoint := server.URL + "/v1/decisions"
	provider, ok := NewDefaultRegistry().Find(endpoint)
	require.True(t, ok)

	input := sampleInput(endpoint, "m")
	outcome, err := provider.Execute(t.Context(), types.SampleKindL7API, input, server.Client())
	require.NoError(t, err)
	require.True(t, outcome.InferenceFallbackRequired)

	inference, err := provider.Execute(t.Context(), types.SampleKindInference, input, server.Client())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, inference.StatusCode)
}

// strictDecisionsUpstream validates the documented request shape the same way
// the dev fake upstream does: predicate questions must use instructions, and
// the non-standard description field is rejected.
func strictDecisionsUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model     string                       `json:"model"`
			Input     json.RawMessage              `json:"input"`
			Questions []map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Model == "" || len(req.Input) == 0 || len(req.Questions) == 0 {
			http.Error(w, "model, input and questions are required", http.StatusBadRequest)
			return
		}
		for _, question := range req.Questions {
			if _, ok := question["description"]; ok {
				http.Error(w, "description is not supported; use instructions", http.StatusBadRequest)
				return
			}
			raw, ok := question["instructions"]
			if !ok {
				http.Error(w, "instructions is required", http.StatusBadRequest)
				return
			}
			var instructions string
			if err := json.Unmarshal(raw, &instructions); err != nil || instructions == "" {
				http.Error(w, "instructions must be a non-empty string", http.StatusBadRequest)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"m","answers":[{"type":"predicate","name":"sample","value":true}],"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
}

func TestDecisionsInferenceSampleAcceptedByStrictUpstream(t *testing.T) {
	server := strictDecisionsUpstream(t)
	defer server.Close()

	endpoint := server.URL + "/v1/decisions"
	provider, ok := NewDefaultRegistry().Find(endpoint)
	require.True(t, ok)

	outcome, err := provider.Execute(t.Context(), types.SampleKindInference, sampleInput(endpoint, "m"), server.Client())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, outcome.StatusCode)
}

func TestDecisionsLegacyDescriptionRejected(t *testing.T) {
	// The pre-alignment sample used "description"; it must fail against a
	// strict OpenAI-compatible upstream so the regression cannot return.
	server := strictDecisionsUpstream(t)
	defer server.Close()

	body := `{"model":"m","input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}],"questions":[{"type":"predicate","name":"sample","description":"Is this a normal customer message?"}]}`
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/v1/decisions", strings.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusBadRequest, response.StatusCode)
}
