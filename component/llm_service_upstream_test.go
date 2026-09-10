package component

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/aigateway/sample"
	aigatewaytypes "opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type policyCapturingSampleProvider struct {
	policy                aigatewaytypes.SampleExecutionPolicy
	capturedDeadline      time.Time
	capturedClientTimeout time.Duration
}

func (*policyCapturingSampleProvider) Supports(string) bool { return true }

func (p *policyCapturingSampleProvider) ExecutionPolicy(aigatewaytypes.SampleKind) (aigatewaytypes.SampleExecutionPolicy, error) {
	return p.policy, nil
}

func (p *policyCapturingSampleProvider) Execute(ctx context.Context, _ aigatewaytypes.SampleKind, input aigatewaytypes.SampleInput, client aigatewaytypes.HTTPDoer) (*aigatewaytypes.SampleExecutionResult, error) {
	p.capturedDeadline, _ = ctx.Deadline()
	p.capturedClientTimeout = client.(*http.Client).Timeout
	return &aigatewaytypes.SampleExecutionResult{
		Request:    &aigatewaytypes.SampleRequest{Endpoint: input.Endpoint, Headers: input.Headers},
		StatusCode: http.StatusOK,
		Status:     "200 OK",
	}, nil
}

func TestValidateHealthCheckEndpoint(t *testing.T) {
	mc := &llmServiceComponentImpl{sampleRegistry: sample.NewDefaultRegistry()}
	require.NoError(t, mc.validateHealthCheckEndpoint("https://api.example.com/v1/embeddings", false))
	require.NoError(t, mc.validateHealthCheckEndpoint("https://api.example.com/v1/chat/completions", true))
	require.NoError(t, mc.validateHealthCheckEndpoint("https://api.example.com/v1/responses", true))

	err := mc.validateHealthCheckEndpoint("https://api.example.com/v1/ocr", true)
	require.Error(t, err)
	customErr, ok := errorx.GetFirstCustomError(err)
	require.True(t, ok)
	require.ErrorIs(t, customErr, errorx.ErrUpstreamHealthCheckNotSupported)

	customRegistry := sample.NewRegistry(&policyCapturingSampleProvider{})
	customComponent := &llmServiceComponentImpl{sampleRegistry: customRegistry}
	require.NoError(t, customComponent.validateHealthCheckEndpoint("https://api.example.com/v1/ocr", true))
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
		"Content-Type":      "application/json",
		"Anthropic-Version": "2023-06-01",
		"Authorization":     "Bearer secret",
		"X-Api-Key":         "key123",
	}
	masked := maskRequestHeaders(headers)
	require.Equal(t, "application/json", masked["Content-Type"])
	require.Equal(t, "2023-06-01", masked["Anthropic-Version"])
	require.Equal(t, maskedAuthSecret, masked["Authorization"])
	require.Equal(t, maskedAuthSecret, masked["X-Api-Key"])
}

func TestSummarizeSampleRequestBodyMultipart(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "audio-model"))
	part, err := writer.CreateFormFile("file", "sample.wav")
	require.NoError(t, err)
	_, err = part.Write([]byte("audio bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	headers := make(http.Header)
	headers.Set("Content-Type", writer.FormDataContentType())
	summary, err := summarizeSampleRequestBody(headers, body.Bytes())
	require.NoError(t, err)
	require.Equal(t, "audio-model", summary["model"])
	fileSummary := summary["file"].(map[string]any)
	require.Equal(t, "sample.wav", fileSummary["filename"])
	require.Equal(t, "application/octet-stream", fileSummary["content_type"])
	require.Equal(t, len("audio bytes"), fileSummary["size"])
}

func TestSummarizeSampleRequestBodyRejectsMalformedMultipart(t *testing.T) {
	headers := make(http.Header)
	headers.Set("Content-Type", "multipart/form-data")
	_, err := summarizeSampleRequestBody(headers, []byte("invalid"))
	require.ErrorContains(t, err, "boundary is missing")
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
	result, err := doUpstreamTest(context.Background(), mustSampleProvider(t, url), upstreamTestParams{url: url, modelName: "gpt-4", authHeaders: map[string]string{"Authorization": "Bearer secret"}})
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
	result, err := doUpstreamTest(context.Background(), mustSampleProvider(t, url), upstreamTestParams{url: url, modelName: "gpt-4o"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.OK)
	require.Contains(t, result.Content, "resp")
}

func TestDoUpstreamTest_MessagesPreservesVersionHeaderAndMasksAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"hello"}]}`))
	}))
	defer srv.Close()
	url := srv.URL + "/v1/messages"
	result, err := doUpstreamTest(context.Background(), mustSampleProvider(t, url), upstreamTestParams{url: url, modelName: "claude-model", authHeaders: map[string]string{"x-api-key": "secret"}})
	require.NoError(t, err)
	var summary requestSummary
	require.NoError(t, json.Unmarshal([]byte(result.Request), &summary))
	require.Equal(t, "2023-06-01", summary.Headers["Anthropic-Version"])
	require.Equal(t, maskedAuthSecret, summary.Headers["X-Api-Key"])
}

func TestDoUpstreamTest_MultipartSummary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"text":"hello"}`))
	}))
	defer srv.Close()
	url := srv.URL + "/v1/audio/transcriptions"
	result, err := doUpstreamTest(context.Background(), mustSampleProvider(t, url), upstreamTestParams{url: url, modelName: "audio-model", authHeaders: map[string]string{"Authorization": "secret"}})
	require.NoError(t, err)
	var summary requestSummary
	require.NoError(t, json.Unmarshal([]byte(result.Request), &summary))
	require.Equal(t, "audio-model", summary.Body["model"])
	fileSummary := summary.Body["file"].(map[string]any)
	require.Equal(t, "sample.wav", fileSummary["filename"])
	require.Equal(t, "audio/wav", fileSummary["content_type"])
	require.NotContains(t, result.Request, "RIFF")
}

func TestDoUpstreamTest_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	url := srv.URL + "/v1/chat/completions"
	result, err := doUpstreamTest(context.Background(), mustSampleProvider(t, url), upstreamTestParams{url: url, modelName: "gpt-4"})
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
	result, err := doUpstreamTest(context.Background(), mustSampleProvider(t, url), upstreamTestParams{url: url, modelName: "gpt-4"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.OK)
	require.NotEmpty(t, result.Error)
}

func TestDoUpstreamTestUsesProviderExecutionPolicy(t *testing.T) {
	provider := &policyCapturingSampleProvider{policy: aigatewaytypes.SampleExecutionPolicy{Timeout: 2 * time.Minute}}
	result, err := doUpstreamTest(context.Background(), provider, upstreamTestParams{url: "https://api.example.com/v1/test", modelName: "model"})
	require.NoError(t, err)
	require.True(t, result.OK)
	require.Equal(t, 2*time.Minute, provider.capturedClientTimeout)
	require.WithinDuration(t, time.Now().Add(2*time.Minute), provider.capturedDeadline, time.Second)
}

func TestDoUpstreamTestRejectsNonPositiveTimeout(t *testing.T) {
	provider := &policyCapturingSampleProvider{}
	_, err := doUpstreamTest(context.Background(), provider, upstreamTestParams{url: "https://api.example.com/v1/test", modelName: "model"})
	require.ErrorContains(t, err, "timeout must be positive")
}

func TestDoUpstreamTest_ASRUsesInputAudio(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	url := srv.URL + "/v1/chat/completions"
	tasks := []string{"auto-speech-recognition"}
	result, err := doUpstreamTest(context.Background(), mustSampleProvider(t, url), upstreamTestParams{url: url, modelName: "qwen3-asr-flash", tasks: tasks})
	require.NoError(t, err)
	require.True(t, result.OK)

	var body struct {
		Messages []struct {
			Content []map[string]any `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(captured, &body))
	require.Len(t, body.Messages, 1)
	require.Len(t, body.Messages[0].Content, 1)
	require.Equal(t, "input_audio", body.Messages[0].Content[0]["type"])
}

func TestLLMServiceComponent_TestUpstream_ASRResolvesTasks(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	ctx := context.TODO()
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().GetByID(ctx, int64(42)).Return(&database.Upstream{
		ID:          42,
		LLMConfigID: 7,
		URL:         srv.URL + "/v1/chat/completions",
		ModelName:   "qwen3-asr-flash",
	}, nil)

	llmConfigStore := mockdatabase.NewMockLLMConfigStore(t)
	llmConfigStore.EXPECT().GetByID(ctx, int64(7)).Return(&database.LLMConfig{
		ID:       7,
		Metadata: map[string]any{"tasks": []any{"auto-speech-recognition"}},
	}, nil)

	mc := &llmServiceComponentImpl{
		upstreamStore:  upstreamStore,
		llmConfigStore: llmConfigStore,
		sampleRegistry: sample.NewDefaultRegistry(),
	}
	result, err := mc.TestUpstream(ctx, &types.TestUpstreamReq{ID: 42})
	require.NoError(t, err)
	require.True(t, result.OK)

	var body struct {
		Messages []struct {
			Content []map[string]any `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(captured, &body))
	require.Len(t, body.Messages, 1)
	require.Equal(t, "input_audio", body.Messages[0].Content[0]["type"])
}

func mustSampleProvider(t *testing.T, url string) aigatewaytypes.SampleProvider {
	t.Helper()
	provider, ok := sample.NewDefaultRegistry().Find(url)
	require.True(t, ok)
	return provider
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
		ID:         42,
		URL:        srv.URL + "/v1/chat/completions",
		ModelName:  "gpt-4",
		AuthHeader: `{"Authorization":"Bearer secret"}`,
	}, nil)

	mc := &llmServiceComponentImpl{
		upstreamStore:  upstreamStore,
		sampleRegistry: sample.NewDefaultRegistry(),
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
		ID:         43,
		URL:        srv.URL + "/v1/responses",
		ModelName:  "gpt-4o",
		AuthHeader: "",
	}, nil)

	mc := &llmServiceComponentImpl{
		upstreamStore:  upstreamStore,
		sampleRegistry: sample.NewDefaultRegistry(),
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
		URL:       "https://api.example.com/v1/ocr",
		ModelName: "text-embedding",
	}, nil)

	mc := &llmServiceComponentImpl{
		upstreamStore:  upstreamStore,
		sampleRegistry: sample.NewDefaultRegistry(),
	}
	_, err := mc.TestUpstream(ctx, &types.TestUpstreamReq{ID: 44})
	require.Error(t, err)
	// The unsupported endpoint error must be an errorx custom error so the
	// handler can map it to a 422 response instead of 500.
	customErr, ok := errorx.GetFirstCustomError(err)
	require.True(t, ok, "expected an errorx custom error for unsupported endpoint")
	require.ErrorIs(t, customErr, errorx.ErrUpstreamConnectionTestNotSupported)
}

func TestLLMServiceComponent_TestUpstreamUsesComponentRegistry(t *testing.T) {
	ctx := context.Background()
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().GetByID(ctx, int64(47)).Return(&database.Upstream{
		ID:        47,
		URL:       "https://api.example.com/v1/custom",
		ModelName: "custom-model",
	}, nil)
	provider := &policyCapturingSampleProvider{
		policy: aigatewaytypes.SampleExecutionPolicy{Timeout: time.Minute},
	}
	mc := &llmServiceComponentImpl{
		upstreamStore:  upstreamStore,
		sampleRegistry: sample.NewRegistry(provider),
	}

	result, err := mc.TestUpstream(ctx, &types.TestUpstreamReq{ID: 47})
	require.NoError(t, err)
	require.True(t, result.OK)
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
