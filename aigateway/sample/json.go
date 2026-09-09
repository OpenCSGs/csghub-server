package sample

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"opencsg.com/csghub-server/aigateway/types"
)

const (
	chatCompletionsRoute           = "/chat/completions"
	responsesRoute                 = "/responses"
	messagesPath                   = "/messages"
	messagesRoute                  = "/v1" + messagesPath
	embeddingsRoute                = "/embeddings"
	rerankRoute                    = "/rerank"
	imageGenerationsRoute          = "/images/generations"
	speechRoute                    = "/audio/speech"
	batchSpeechRoute               = "/audio/speech/batch"
	videoGenerationsRoute          = "/video/generations"
	responsesSampleMaxOutputTokens = 16
	defaultAnthropicVersion        = "2023-06-01"
)

func jsonRequest(input types.SampleInput, dto any) (*types.SampleRequest, error) {
	body, err := json.Marshal(dto)
	if err != nil {
		return nil, fmt.Errorf("marshal inference sample: %w", err)
	}
	headers := cloneSampleHeaders(input.Headers)
	headers.Set("Content-Type", "application/json")
	return &types.SampleRequest{
		Method:   http.MethodPost,
		Endpoint: input.Endpoint,
		Headers:  headers,
		Body:     body,
	}, nil
}

func modelsL7Request(route string) requestFactory {
	return func(input types.SampleInput) (*types.SampleRequest, error) {
		baseURL, err := endpointBaseURL(input.Endpoint, route)
		if err != nil {
			return nil, err
		}
		return &types.SampleRequest{
			Method:   http.MethodGet,
			Endpoint: baseURL + "/models",
			Headers:  cloneSampleHeaders(input.Headers),
		}, nil
	}
}

func chatCompletionsRequest(input types.SampleInput) (*types.SampleRequest, error) {
	dto := types.ChatCompletionRequest{
		Model: input.Model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(sampleText(input)),
		},
		MaxTokens: 1,
		Stream:    false,
	}
	return jsonRequest(input, dto)
}

func responsesRequest(input types.SampleInput) (*types.SampleRequest, error) {
	text, err := json.Marshal(sampleText(input))
	if err != nil {
		return nil, fmt.Errorf("marshal responses sample input: %w", err)
	}
	maxOutputTokens := responsesSampleMaxOutputTokens
	dto := types.ResponsesRequest{
		Model:           input.Model,
		Input:           text,
		MaxOutputTokens: &maxOutputTokens,
		Stream:          false,
	}
	return jsonRequest(input, dto)
}

func withDefaultAnthropicVersion(input types.SampleInput) types.SampleInput {
	input.Headers = cloneSampleHeaders(input.Headers)
	if input.Headers.Get("anthropic-version") == "" {
		input.Headers.Set("anthropic-version", defaultAnthropicVersion)
	}
	return input
}

func messagesModelsL7Request(input types.SampleInput) (*types.SampleRequest, error) {
	return modelsL7Request(messagesPath)(withDefaultAnthropicVersion(input))
}

func messagesRequest(input types.SampleInput) (*types.SampleRequest, error) {
	content, err := json.Marshal(sampleText(input))
	if err != nil {
		return nil, fmt.Errorf("marshal messages sample content: %w", err)
	}
	dto := types.AnthropicMessagesRequest{
		Model: input.Model,
		Messages: []types.AnthropicMessage{
			{Role: "user", Content: content},
		},
		MaxTokens: 1,
		Stream:    false,
	}
	return jsonRequest(withDefaultAnthropicVersion(input), dto)
}

func embeddingsRequest(input types.SampleInput) (*types.SampleRequest, error) {
	dto := types.EmbeddingRequest{EmbeddingNewParams: openai.EmbeddingNewParams{
		Model: input.Model,
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: param.NewOpt(sampleText(input)),
		},
	}}
	return jsonRequest(input, dto)
}

func rerankRequest(input types.SampleInput) (*types.SampleRequest, error) {
	dto := types.RerankRequest{
		Model:     input.Model,
		Query:     sampleText(input),
		Documents: []string{sampleText(input)},
	}
	return jsonRequest(input, dto)
}

func imageGenerationsRequest(input types.SampleInput) (*types.SampleRequest, error) {
	dto := types.ImageGenerationRequest{ImageGenerateParams: openai.ImageGenerateParams{
		Model:  input.Model,
		Prompt: sampleText(input),
	}}
	return jsonRequest(input, dto)
}

func speechRequest(input types.SampleInput) (*types.SampleRequest, error) {
	dto := types.SpeechRequest{Model: input.Model, Input: sampleText(input), Stream: false}
	return jsonRequest(input, dto)
}

func batchSpeechRequest(input types.SampleInput) (*types.SampleRequest, error) {
	item, err := json.Marshal(struct {
		Input string `json:"input"`
	}{Input: sampleText(input)})
	if err != nil {
		return nil, fmt.Errorf("marshal batch speech sample item: %w", err)
	}
	dto := types.BatchSpeechRequest{Model: input.Model, Items: []json.RawMessage{item}}
	return jsonRequest(input, dto)
}

func videoGenerationsRequest(input types.SampleInput) (*types.SampleRequest, error) {
	dto := types.VideoGenerationRequest{Model: input.Model, Prompt: sampleText(input)}
	return jsonRequest(input, dto)
}
