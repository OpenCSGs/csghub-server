package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	commonType "opencsg.com/csghub-server/common/types"

	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/token"
)

type chatCounterAttemptConfigurator interface {
	SetCreateParam(param token.CreateParam)
}

type llmLogModelConfigurator interface {
	SetModelID(modelID string)
	SetProvider(provider string)
}

func marshalChatRequestBody(chatReq *ChatCompletionRequest, modelName string) ([]byte, error) {
	if chatReq == nil {
		return nil, fmt.Errorf("chat request is nil")
	}
	reqCopy := *chatReq
	reqCopy.Model = modelName
	body, err := json.Marshal(&reqCopy)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}
	return body, nil
}

func updateChatAttemptRuntime(tokenCounter token.ChatTokenCounter, logCapture component.LLMLogRecorder, modelTarget *resolvedModelTarget) {
	if tokenCounterConfigurer, ok := tokenCounter.(chatCounterAttemptConfigurator); ok {
		tokenCounterConfigurer.SetCreateParam(token.CreateParam{
			Endpoint: modelTarget.Target,
			Host:     modelTarget.Host,
			Model:    modelTarget.ModelName,
			ImageID:  modelTarget.Model.ImageID,
			Provider: modelTarget.Model.Provider,
		})
	}
	if logCaptureConfigurer, ok := logCapture.(llmLogModelConfigurator); ok {
		logCaptureConfigurer.SetModelID(modelTarget.ModelName)
		logCaptureConfigurer.SetProvider(modelTarget.Model.Provider)
	}
}

// applyChatCompletionsEndpointCompatibility keeps /v1/chat/completions usable
// when an upstream is configured with a sibling /responses endpoint. This is a
// URL compatibility shortcut only; it does not adapt Chat payloads to Responses.
func applyChatCompletionsEndpointCompatibility(ctx context.Context, modelTarget *resolvedModelTarget) {
	rewritten, ok := rewriteResponsesURLToChatCompletions(modelTarget.Target)
	if !ok {
		return
	}
	original := modelTarget.Target
	modelTarget.Target = rewritten
	modelTarget.Model.Endpoint = rewritten
	modelTarget.Upstream.URL = rewritten
	slog.InfoContext(ctx, "rewrite responses upstream url for chat completions request",
		slog.String("api", "/v1/chat/completions"),
		slog.String("original_upstream_url", original),
		slog.String("rewritten_upstream_url", rewritten),
		slog.String("compatibility", "url_path_shortcut_not_chat_responses_adapter"),
		slog.String("reason", "upstream_url_responses_to_chat_completions"))
}

func rewriteResponsesURLToChatCompletions(rawURL string) (string, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return rawURL, false
	}
	path := strings.TrimRight(parsed.Path, "/")
	if !responsespkg.PathEndsWithSegments(path, "responses") {
		return rawURL, false
	}
	parts := strings.Split(path, "/")
	parts[len(parts)-1] = "chat/completions"
	parsed.Path = strings.Join(parts, "/")
	return parsed.String(), true
}

func applyChatFallbackTarget(ctx context.Context, headers http.Header, modelTarget *resolvedModelTarget, upstream commonType.UpstreamConfig, tokenCounter token.ChatTokenCounter, logCapture component.LLMLogRecorder) {
	targetURL := upstream.URL
	modelTarget.Target = targetURL
	modelTarget.Model.Endpoint = targetURL
	modelTarget.Upstream = upstream
	modelTarget.ModelName = resolveEndpointModelName(modelTarget.Model.ID, upstream)
	// orverride models's AuthHead, Provider with upstream
	applyEndpointOverrides(modelTarget.Model, modelTarget.Upstream)
	if err := applyModelAuthHeaders(headers, modelTarget.Model); err != nil {
		slog.WarnContext(ctx, "invalid fallback auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}
	applyChatCompletionsEndpointCompatibility(ctx, modelTarget)
	updateChatAttemptRuntime(tokenCounter, logCapture, modelTarget)
}
