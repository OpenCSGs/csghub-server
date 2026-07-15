package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
	commontypes "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/trace"
)

func newSpeechRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestOpenAIHandler_Speech(t *testing.T) {
	t.Run("successful binary passthrough with rewritten model", func(t *testing.T) {
		tester, c, w := setupTest(t)

		var downstreamBody map[string]any
		var downstreamAuth string
		audioBytes := []byte("RIFF-fake-wav-bytes")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/audio/speech", r.URL.Path)
			downstreamAuth = r.Header.Get("Authorization")
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &downstreamBody))
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write(audioBytes)
		}))
		defer server.Close()

		c.Request = newSpeechRequest(t, `{"model":"model1","input":"hello","voice":"vivian","language":"English"}`)
		c.Set(trace.HeaderRequestID, "req-speech-ok")
		recorder := &testGenerationRecorderWithMutex{}
		tracer := &testLLMTracerWithMutex{recorder: recorder}
		tester.handler.llmTracer = tracer
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "backend-model",
				Object:   "model",
				Metadata: map[string]any{},
				Task:     string(commontypes.TextToAudio),
			},
			Endpoint: server.URL + "/v1/audio/speech",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: server.URL + "/v1/audio/speech", Enabled: true, ModelName: "backend-model"},
			},
			ExternalModelInfo: types.ExternalModelInfo{
				AuthHead: `{"Authorization":"Bearer provider-token"}`,
			},
		}

		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "hello", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, mock.Anything, mock.MatchedBy(func(usage *token.Usage) bool {
			// Binary responses fall back to input-character billing.
			return usage != nil &&
				usage.CompletionTokens == int64(len("hello")) &&
				usage.DataType == string(commontypes.DataTypeAudio)
		}), "").RunAndReturn(
			func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
				wg.Done()
				return nil
			}).Once()

		tester.handler.Speech(c)
		wg.Wait()

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, audioBytes, w.Body.Bytes())
		require.Equal(t, "backend-model", downstreamBody["model"])
		require.Equal(t, "hello", downstreamBody["input"])
		require.Equal(t, "vivian", downstreamBody["voice"])
		// Extension fields are passed through unchanged.
		require.Equal(t, "English", downstreamBody["language"])
		require.Equal(t, "Bearer provider-token", downstreamAuth)
		starts := tracer.Starts()
		require.Len(t, starts, 1)
		require.Equal(t, "req-speech-ok", starts[0].RequestID)
		require.Equal(t, "generate_content", starts[0].OperationName)
		require.Equal(t, "audio", starts[0].Metadata[llmtrace.TraceMetadataKeyGenAIOutputType])
	})

	t.Run("sse stream captures usage from speech.audio.done", func(t *testing.T) {
		tester, c, w := setupTest(t)

		sseBody := "event: speech.audio.delta\n" +
			`data: {"type":"speech.audio.delta","audio":"UklGRg==","response_format":"pcm"}` + "\n\n" +
			"event: speech.audio.done\n" +
			`data: {"type":"speech.audio.done","usage":{"input_tokens":119,"output_tokens":77,"total_tokens":196}}` + "\n\n"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(sseBody))
		}))
		defer server.Close()

		c.Request = newSpeechRequest(t, `{"model":"model1","input":"hello","stream":true,"response_format":"pcm"}`)
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "backend-model",
				Object:   "model",
				Metadata: map[string]any{},
				Task:     "text-to-speech",
			},
			Endpoint: server.URL + "/v1/audio/speech",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: server.URL + "/v1/audio/speech", Enabled: true, ModelName: "backend-model"},
			},
		}

		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "hello", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, mock.Anything, mock.MatchedBy(func(usage *token.Usage) bool {
			return usage != nil &&
				usage.TotalTokens == 196 &&
				usage.PromptTokens == 119 &&
				usage.CompletionTokens == 77 &&
				usage.DataType == string(commontypes.DataTypeAudio)
		}), "").RunAndReturn(
			func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
				wg.Done()
				return nil
			}).Once()

		tester.handler.Speech(c)
		wg.Wait()

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, sseBody, w.Body.String())
	})

	t.Run("missing model", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newSpeechRequest(t, `{"input":"hello"}`)

		tester.handler.Speech(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Model and input cannot be empty")
	})

	t.Run("missing input", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newSpeechRequest(t, `{"model":"model1"}`)

		tester.handler.Speech(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Model and input cannot be empty")
	})

	t.Run("model not found", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newSpeechRequest(t, `{"model":"missing-model","input":"hello"}`)

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "missing-model").Return(nil, nil).Once()

		tester.handler.Speech(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "model_not_found")
	})

	t.Run("rejects incompatible model task", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newSpeechRequest(t, `{"model":"chat-model","input":"hello"}`)
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "chat-model",
				Object:   "model",
				Task:     string(commontypes.TextGeneration),
				Metadata: map[string]any{},
			},
			Endpoint: "https://api.example.com/v1/chat/completions",
			Upstreams: []commontypes.UpstreamConfig{{
				URL:       "https://api.example.com/v1/chat/completions",
				Enabled:   true,
				ModelName: "chat-model",
			}},
		}

		tester.mocks.openAIComp.EXPECT().
			GetModelByID(mock.Anything, "testuser", "chat-model").
			Return(model, nil).
			Once()

		tester.handler.Speech(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "model_task_mismatch")
	})

	t.Run("sensitive input rejected", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newSpeechRequest(t, `{"model":"model1","input":"bad text"}`)
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "backend-model",
				Object:   "model",
				Metadata: map[string]any{},
				Task:     "text-to-speech",
			},
			Endpoint: "https://api.example.com/v1/audio/speech",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: "https://api.example.com/v1/audio/speech", Enabled: true, ModelName: "backend-model"},
			},
		}

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "bad text", "testuuid").Return(&rpc.CheckResult{IsSensitive: true}, nil).Once()

		tester.handler.Speech(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "content_policy_violation")
	})
}

func newSpeechBatchRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestOpenAIHandler_SpeechBatch(t *testing.T) {
	t.Run("successful batch sums usage of successful items", func(t *testing.T) {
		tester, c, w := setupTest(t)

		var downstreamBody map[string]any
		batchResp := `{"results":[` +
			`{"status":"success","usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}},` +
			`{"status":"error","error":"boom"},` +
			`{"status":"success","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}` +
			`]}`
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/audio/speech/batch", r.URL.Path)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &downstreamBody))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(batchResp))
		}))
		defer server.Close()

		c.Request = newSpeechBatchRequest(t, `{"model":"model1","items":[{"input":"hello"},{"input":"world"},{"input":"again"}],"response_format":"wav"}`)
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "backend-model",
				Object:   "model",
				Metadata: map[string]any{},
				Task:     "text-to-speech",
			},
			Endpoint: server.URL + "/v1/audio/speech",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: server.URL + "/v1/audio/speech", Enabled: true, ModelName: "backend-model"},
			},
		}

		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "hello\nworld\nagain", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, mock.Anything, mock.MatchedBy(func(usage *token.Usage) bool {
			return usage != nil &&
				usage.PromptTokens == 11 &&
				usage.CompletionTokens == 22 &&
				usage.TotalTokens == 33 &&
				usage.CompletionRC == 2 &&
				usage.DataType == string(commontypes.DataTypeAudio)
		}), "").RunAndReturn(
			func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
				wg.Done()
				return nil
			}).Once()

		tester.handler.SpeechBatch(c)
		wg.Wait()

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, batchResp, w.Body.String())
		require.Equal(t, "backend-model", downstreamBody["model"])
		require.Equal(t, "wav", downstreamBody["response_format"])
		items, ok := downstreamBody["items"].([]any)
		require.True(t, ok)
		require.Len(t, items, 3)
	})

	t.Run("response without usage falls back to input characters", func(t *testing.T) {
		tester, c, w := setupTest(t)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[{"status":"error","error":"boom"}]}`))
		}))
		defer server.Close()

		c.Request = newSpeechBatchRequest(t, `{"model":"model1","items":[{"input":"hello"},{"input":"world"}]}`)
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "backend-model",
				Object:   "model",
				Metadata: map[string]any{},
				Task:     "text-to-speech",
			},
			Endpoint: server.URL + "/v1/audio/speech",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: server.URL + "/v1/audio/speech", Enabled: true, ModelName: "backend-model"},
			},
		}

		var wg sync.WaitGroup
		wg.Add(1)
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "hello\nworld", "testuuid").Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, mock.Anything, mock.MatchedBy(func(usage *token.Usage) bool {
			return usage != nil &&
				usage.CompletionTokens == int64(len("helloworld")) &&
				usage.DataType == string(commontypes.DataTypeAudio)
		}), "").RunAndReturn(
			func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
				wg.Done()
				return nil
			}).Once()

		tester.handler.SpeechBatch(c)
		wg.Wait()

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	})

	t.Run("missing items", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newSpeechBatchRequest(t, `{"model":"model1","items":[]}`)

		tester.handler.SpeechBatch(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Model and items cannot be empty")
	})

	t.Run("too many items rejected", func(t *testing.T) {
		tester, c, w := setupTest(t)
		items := make([]string, maxSpeechBatchItems+1)
		for i := range items {
			items[i] = `{"input":"hi"}`
		}
		body := `{"model":"model1","items":[` + strings.Join(items, ",") + `]}`
		c.Request = newSpeechBatchRequest(t, body)

		tester.handler.SpeechBatch(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "too many items")
	})

	t.Run("total input too long rejected", func(t *testing.T) {
		tester, c, w := setupTest(t)
		longInput := strings.Repeat("a", maxSpeechBatchInputChars+1)
		c.Request = newSpeechBatchRequest(t, `{"model":"model1","items":[{"input":"`+longInput+`"}]}`)

		tester.handler.SpeechBatch(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "total input text too long")
	})

	t.Run("sensitive input rejected", func(t *testing.T) {
		tester, c, w := setupTest(t)
		c.Request = newSpeechBatchRequest(t, `{"model":"model1","items":[{"input":"bad text"}]}`)
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:       "backend-model",
				Object:   "model",
				Metadata: map[string]any{},
				Task:     "text-to-speech",
			},
			Endpoint: "https://api.example.com/v1/audio/speech",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: "https://api.example.com/v1/audio/speech", Enabled: true, ModelName: "backend-model"},
			},
		}

		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
		tester.mocks.openAIComp.EXPECT().CheckBalance(mock.Anything, "testuuid").Return(nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckImagePrompts(mock.Anything, "bad text", "testuuid").Return(&rpc.CheckResult{IsSensitive: true}, nil).Once()

		tester.handler.SpeechBatch(c)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "content_policy_violation")
	})
}

func TestSpeechBatchProxyPath(t *testing.T) {
	ctx := context.Background()
	require.Equal(t, "", speechBatchProxyPath(ctx, ""))
	require.Equal(t, "", speechBatchProxyPath(ctx, "https://host"))
	require.Equal(t, "", speechBatchProxyPath(ctx, "https://host/"))
	require.Equal(t, "/v1/audio/speech/batch", speechBatchProxyPath(ctx, "https://host/v1/audio/speech"))
	require.Equal(t, "/v1/audio/speech/batch", speechBatchProxyPath(ctx, "https://host/v1/audio/speech/batch"))
}

func TestSpeechRequest_JSONRoundTrip(t *testing.T) {
	body := `{"model":"m","input":"hi","voice":"vivian","speed":1.5,"task_type":"Base","ref_audio":"https://example.com/a.wav"}`
	var req SpeechRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))
	require.Equal(t, "m", req.Model)
	require.Equal(t, "hi", req.Input)
	require.Equal(t, "vivian", req.Voice)
	require.Equal(t, 1.5, req.Speed)

	req.Model = "backend"
	out, err := json.Marshal(req)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	require.Equal(t, "backend", m["model"])
	require.Equal(t, "Base", m["task_type"])
	require.Equal(t, "https://example.com/a.wav", m["ref_audio"])
	_, hasStream := m["stream"]
	require.False(t, hasStream)
}
