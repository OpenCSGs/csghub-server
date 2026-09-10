package sample

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestJSONFactoriesBuildDTOBodies(t *testing.T) {
	text := "sample text"
	maxOutputTokens := responsesSampleMaxOutputTokens
	content, err := json.Marshal(text)
	require.NoError(t, err)
	batchItem, err := json.Marshal(struct {
		Input string `json:"input"`
	}{Input: text})
	require.NoError(t, err)
	tests := []struct {
		name    string
		factory requestFactory
		dto     any
	}{
		{name: "chat", factory: chatCompletionsRequest, dto: types.ChatCompletionRequest{Model: "model", Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage(text)}, MaxTokens: 1}},
		{name: "responses", factory: responsesRequest, dto: types.ResponsesRequest{Model: "model", Input: content, MaxOutputTokens: &maxOutputTokens}},
		{name: "messages", factory: messagesRequest, dto: types.AnthropicMessagesRequest{Model: "model", Messages: []types.AnthropicMessage{{Role: "user", Content: content}}, MaxTokens: 1}},
		{name: "rerank", factory: rerankRequest, dto: types.RerankRequest{Model: "model", Query: text, Documents: []string{text}}},
		{name: "image generation", factory: imageGenerationsRequest, dto: types.ImageGenerationRequest{ImageGenerateParams: openai.ImageGenerateParams{Model: "model", Prompt: text}}},
		{name: "speech", factory: speechRequest, dto: types.SpeechRequest{Model: "model", Input: text}},
		{name: "batch speech", factory: batchSpeechRequest, dto: types.BatchSpeechRequest{Model: "model", Items: []json.RawMessage{batchItem}}},
		{name: "video generation", factory: videoGenerationsRequest, dto: types.VideoGenerationRequest{Model: "model", Prompt: text}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := sampleInput("https://api.example.com/v1/test", "model")
			input.Text = text
			request, err := test.factory(input)
			require.NoError(t, err)
			want, err := json.Marshal(test.dto)
			require.NoError(t, err)
			require.JSONEq(t, string(want), string(request.Body))
			require.Equal(t, http.MethodPost, request.Method)
			require.Equal(t, "application/json", request.Headers.Get("Content-Type"))
			require.Equal(t, "Bearer test-token", request.Headers.Get("Authorization"))
		})
	}
}

func TestEmbeddingsRequest(t *testing.T) {
	request, err := embeddingsRequest(sampleInput("https://api.example.com/v1/embeddings", "embedding-model"))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(request.Body, &body))
	require.Equal(t, "embedding-model", body["model"])
	require.Equal(t, "hi", body["input"])
}

func TestJSONRequestAcceptsNilHeaders(t *testing.T) {
	request, err := responsesRequest(types.SampleInput{Endpoint: "https://api.example.com/v1/responses", Model: "model"})
	require.NoError(t, err)
	require.Equal(t, "application/json", request.Headers.Get("Content-Type"))
}

func TestModelsL7RequestPreservesHeaders(t *testing.T) {
	input := sampleInput("https://api.example.com/prefix/v1/embeddings", "model")
	request, err := modelsL7Request(embeddingsRoute)(input)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, request.Method)
	require.Equal(t, "https://api.example.com/prefix/v1/models", request.Endpoint)
	require.Equal(t, "Bearer test-token", request.Headers.Get("Authorization"))
}

func TestMessagesRequestHeadersAndL7(t *testing.T) {
	endpoint := "https://api.anthropic.com/v1/messages"
	provider, ok := NewDefaultRegistry().Find(endpoint)
	require.True(t, ok)
	input := sampleInput(endpoint, "claude-model")
	input.Headers.Set("x-api-key", "test-key")
	l7, err := sampleRequestBuilder(t, provider).Build(types.SampleKindL7API, input)
	require.NoError(t, err)
	inference, err := sampleRequestBuilder(t, provider).Build(types.SampleKindInference, input)
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, l7.Method)
	require.Equal(t, "https://api.anthropic.com/v1/models", l7.Endpoint)
	require.Equal(t, "test-key", l7.Headers.Get("x-api-key"))
	require.Equal(t, defaultAnthropicVersion, l7.Headers.Get("anthropic-version"))
	require.Equal(t, defaultAnthropicVersion, inference.Headers.Get("anthropic-version"))
	input.Headers.Set("anthropic-version", "custom-version")
	overriddenL7, err := messagesModelsL7Request(input)
	require.NoError(t, err)
	require.Equal(t, "custom-version", overriddenL7.Headers.Get("anthropic-version"))
	overridden, err := messagesRequest(input)
	require.NoError(t, err)
	require.Equal(t, "custom-version", overridden.Headers.Get("anthropic-version"))
}

func TestResponsesL7UsesModelsEndpoint(t *testing.T) {
	endpoint := "https://dashscope.example/compatible-mode/v1/responses"
	provider, ok := NewDefaultRegistry().Find(endpoint)
	require.True(t, ok)
	request, err := sampleRequestBuilder(t, provider).Build(types.SampleKindL7API, sampleInput(endpoint, "model"))
	require.NoError(t, err)
	require.Equal(t, http.MethodGet, request.Method)
	require.Equal(t, "https://dashscope.example/compatible-mode/v1/models", request.Endpoint)
}

func TestConcurrentBuildsDoNotLeakState(t *testing.T) {
	provider, ok := NewDefaultRegistry().Find("https://api.example.com/v1/chat/completions")
	require.True(t, ok)
	builder := sampleRequestBuilder(t, provider)
	var waitGroup sync.WaitGroup
	for index := 0; index < 20; index++ {
		index := index
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			model := "model-" + strconv.Itoa(index)
			request, err := builder.Build(types.SampleKindInference, sampleInput("https://api.example.com/v1/chat/completions", model))
			require.NoError(t, err)
			var body types.ChatCompletionRequest
			require.NoError(t, json.Unmarshal(request.Body, &body))
			require.Equal(t, model, body.Model)
		}()
	}
	waitGroup.Wait()
}

func TestChatCompletionsAudioRequestUsesInputAudio(t *testing.T) {
	input := sampleInput("https://api.example.com/v1/chat/completions", "qwen3-asr-flash")
	input.Tasks = []string{"auto-speech-recognition"}

	request, err := chatCompletionsRequest(input)
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, request.Method)
	require.Equal(t, "application/json", request.Headers.Get("Content-Type"))

	var body struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Type       string         `json:"type"`
				InputAudio map[string]any `json:"input_audio"`
			} `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(request.Body, &body))
	require.Equal(t, "qwen3-asr-flash", body.Model)
	require.Len(t, body.Messages, 1)
	require.Equal(t, "user", body.Messages[0].Role)
	require.Len(t, body.Messages[0].Content, 1)

	part := body.Messages[0].Content[0]
	require.Equal(t, "input_audio", part.Type)
	data, ok := part.InputAudio["data"].(string)
	require.True(t, ok)
	require.True(t, strings.HasPrefix(data, "data:audio/wav;base64,"), data)
	_, hasFormat := part.InputAudio["format"]
	require.False(t, hasFormat, "input_audio must not include a separate format field")
}

func TestChatCompletionsRequestRemainsTextForNonASR(t *testing.T) {
	request, err := chatCompletionsRequest(sampleInput("https://api.example.com/v1/chat/completions", "text-model"))
	require.NoError(t, err)
	var body struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(request.Body, &body))
	require.Len(t, body.Messages, 1)
	require.Equal(t, "hi", body.Messages[0].Content)
}

func sampleRequestBuilder(t *testing.T, provider types.SampleProvider) types.SampleRequestBuilder {
	t.Helper()
	builder, ok := provider.(types.SampleRequestBuilder)
	require.True(t, ok)
	return builder
}

func sampleInput(endpoint, model string) types.SampleInput {
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer test-token")
	return types.SampleInput{Endpoint: endpoint, Headers: headers, Model: model}
}
