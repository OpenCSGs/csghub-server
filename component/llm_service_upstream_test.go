package component

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestDetectEndpointKind(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    testEndpointKind
	}{
		{"chat completions", "https://api.example.com/v1/chat/completions", endpointKindChatCompletions},
		{"chat completions trailing slash", "https://api.example.com/v1/chat/completions/", endpointKindChatCompletions},
		{"chat completions with query", "https://api.example.com/v1/chat/completions?foo=bar", endpointKindChatCompletions},
		{"responses", "https://api.example.com/v1/responses", endpointKindResponses},
		{"responses trailing slash", "https://api.example.com/v1/responses/", endpointKindResponses},
		{"responses with fragment", "https://api.example.com/v1/responses#frag", endpointKindResponses},
		{"unsupported root", "https://api.example.com/v1", endpointKindUnsupported},
		{"unsupported embeddings", "https://api.example.com/v1/embeddings", endpointKindUnsupported},
		{"empty", "", endpointKindUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, detectEndpointKind(tt.url))
		})
	}
}

func TestParseAuthHeader(t *testing.T) {
	t.Run("empty returns empty map", func(t *testing.T) {
		headers, err := parseAuthHeader("")
		require.NoError(t, err)
		require.Empty(t, headers)
	})
	t.Run("json object", func(t *testing.T) {
		headers, err := parseAuthHeader(`{"Authorization":"Bearer secret","X-Api-Key":"key123"}`)
		require.NoError(t, err)
		require.Equal(t, "Bearer secret", headers["Authorization"])
		require.Equal(t, "key123", headers["X-Api-Key"])
	})
	t.Run("bare bearer string", func(t *testing.T) {
		headers, err := parseAuthHeader("Bearer mytoken")
		require.NoError(t, err)
		require.Equal(t, "Bearer mytoken", headers["Authorization"])
	})
	t.Run("whitespace only", func(t *testing.T) {
		headers, err := parseAuthHeader("   ")
		require.NoError(t, err)
		require.Empty(t, headers)
	})
}

func TestMaskRequestHeaders(t *testing.T) {
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer secret",
		"X-Api-Key":     "key123",
	}
	masked := maskRequestHeaders(headers)
	require.Equal(t, "application/json", masked["Content-Type"])
	require.Equal(t, maskedAuthSecret, masked["Authorization"])
	require.Equal(t, maskedAuthSecret, masked["X-Api-Key"])
}

func TestBuildTestRequestBody(t *testing.T) {
	t.Run("chat completions", func(t *testing.T) {
		body, err := buildTestRequestBody(endpointKindChatCompletions, "gpt-4")
		require.NoError(t, err)
		require.Equal(t, "gpt-4", body["model"])
		require.Equal(t, false, body["stream"])
		require.NotContains(t, body, "max_tokens")
		messages, ok := body["messages"].([]map[string]string)
		require.True(t, ok)
		require.Len(t, messages, 1)
		require.Equal(t, "hi", messages[0]["content"])
	})
	t.Run("responses", func(t *testing.T) {
		body, err := buildTestRequestBody(endpointKindResponses, "gpt-4o")
		require.NoError(t, err)
		require.Equal(t, "gpt-4o", body["model"])
		require.Equal(t, "hi", body["input"])
		require.Equal(t, false, body["stream"])
		require.NotContains(t, body, "max_output_tokens")
	})
	t.Run("unsupported returns error", func(t *testing.T) {
		_, err := buildTestRequestBody(endpointKindUnsupported, "model")
		require.Error(t, err)
	})
}

func TestDoUpstreamTest_ChatCompletionsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer srv.Close()

	url := srv.URL + "/v1/chat/completions"
	client := &http.Client{}
	result, err := doUpstreamTest(context.Background(), client, url, endpointKindChatCompletions, "gpt-4", map[string]string{"Authorization": "Bearer secret"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.Equal(t, http.StatusOK, result.Status)
	// Content is the raw upstream response, not parsed
	require.Contains(t, result.Content, "hello")

	// Verify the request summary is masked
	var summary requestSummary
	require.NoError(t, json.Unmarshal([]byte(result.Request), &summary))
	require.Equal(t, url, summary.URL)
	require.Equal(t, http.MethodPost, summary.Method)
	require.Equal(t, maskedAuthSecret, summary.Headers["Authorization"])
	require.Equal(t, "application/json", summary.Headers["Content-Type"])
}

func TestDoUpstreamTest_ResponsesSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"resp"}]}]}`))
	}))
	defer srv.Close()

	url := srv.URL + "/v1/responses"
	client := &http.Client{}
	result, err := doUpstreamTest(context.Background(), client, url, endpointKindResponses, "gpt-4o", map[string]string{})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.Contains(t, result.Content, "resp")
}

func TestDoUpstreamTest_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	url := srv.URL + "/v1/chat/completions"
	client := &http.Client{}
	result, err := doUpstreamTest(context.Background(), client, url, endpointKindChatCompletions, "gpt-4", map[string]string{})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.OK)
	require.Equal(t, http.StatusUnauthorized, result.Status)
	// Content is the raw upstream response including errors
	require.Contains(t, result.Content, "invalid api key")
}

func TestDoUpstreamTest_NetworkError(t *testing.T) {
	// Use a closed server to simulate a connection error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	url := srv.URL + "/v1/chat/completions"
	client := &http.Client{}
	result, err := doUpstreamTest(context.Background(), client, url, endpointKindChatCompletions, "gpt-4", map[string]string{})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.OK)
	require.NotEmpty(t, result.Error)
}

func TestLLMServiceComponent_TestUpstream_ChatCompletions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	ctx := context.TODO()
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().GetByID(ctx, int64(42)).Return(&database.Upstream{
		ID:        42,
		URL:       srv.URL + "/v1/chat/completions",
		ModelName: "gpt-4",
		AuthHeader: `{"Authorization":"Bearer secret"}`,
	}, nil)

	mc := &llmServiceComponentImpl{
		upstreamStore: upstreamStore,
	}
	result, err := mc.TestUpstream(ctx, &types.TestUpstreamReq{ID: 42})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.Contains(t, result.Content, "ok")
}

func TestLLMServiceComponent_TestUpstream_Responses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"resp"}]}]}`))
	}))
	defer srv.Close()

	ctx := context.TODO()
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().GetByID(ctx, int64(43)).Return(&database.Upstream{
		ID:        43,
		URL:       srv.URL + "/v1/responses",
		ModelName: "gpt-4o",
		AuthHeader: "",
	}, nil)

	mc := &llmServiceComponentImpl{
		upstreamStore: upstreamStore,
	}
	result, err := mc.TestUpstream(ctx, &types.TestUpstreamReq{ID: 43})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.Contains(t, result.Content, "resp")
}

func TestLLMServiceComponent_TestUpstream_UnsupportedEndpoint(t *testing.T) {
	ctx := context.TODO()
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().GetByID(ctx, int64(44)).Return(&database.Upstream{
		ID:        44,
		URL:       "https://api.example.com/v1/embeddings",
		ModelName: "text-embedding",
	}, nil)

	mc := &llmServiceComponentImpl{
		upstreamStore: upstreamStore,
	}
	_, err := mc.TestUpstream(ctx, &types.TestUpstreamReq{ID: 44})
	require.Error(t, err)
	// The unsupported endpoint error must be an errorx custom error so the
	// handler can map it to a 422 response instead of 500.
	customErr, ok := errorx.GetFirstCustomError(err)
	require.True(t, ok, "expected an errorx custom error for unsupported endpoint")
	require.ErrorIs(t, customErr, errorx.ErrReqParamInvalid)
}

func TestLLMServiceComponent_TestUpstream_EmptyURL(t *testing.T) {
	ctx := context.TODO()
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().GetByID(ctx, int64(45)).Return(&database.Upstream{
		ID:        45,
		URL:       "  ",
		ModelName: "gpt-4",
	}, nil)

	mc := &llmServiceComponentImpl{
		upstreamStore: upstreamStore,
	}
	_, err := mc.TestUpstream(ctx, &types.TestUpstreamReq{ID: 45})
	require.Error(t, err)
}

func TestLLMServiceComponent_TestUpstream_EmptyModelName(t *testing.T) {
	ctx := context.TODO()
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().GetByID(ctx, int64(46)).Return(&database.Upstream{
		ID:        46,
		URL:       "https://api.example.com/v1/chat/completions",
		ModelName: "",
	}, nil)

	mc := &llmServiceComponentImpl{
		upstreamStore: upstreamStore,
	}
	_, err := mc.TestUpstream(ctx, &types.TestUpstreamReq{ID: 46})
	require.Error(t, err)
}

func TestLLMServiceComponent_TestUpstream_NotFound(t *testing.T) {
	ctx := context.TODO()
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().GetByID(ctx, int64(99)).Return(nil, fmt.Errorf("record not found"))

	mc := &llmServiceComponentImpl{
		upstreamStore: upstreamStore,
	}
	_, err := mc.TestUpstream(ctx, &types.TestUpstreamReq{ID: 99})
	require.Error(t, err)
}
