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
	"opencsg.com/csghub-server/aigateway/types"
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

func marshalChatRequestBody(chatReq *types.ChatCompletionRequest, modelName string) ([]byte, error) {
	if chatReq == nil {
		return nil, fmt.Errorf("chat request is nil")
	}
	// Native chat requests keep the client's original bytes: proxy them with
	// only the fields the gateway must override, so vendor-specific fields
	// inside messages or tools (e.g. reasoning_content, cache_control)
	// survive the round trip unchanged.
	if len(chatReq.RawBody) > 0 {
		// Fast path: the client's model name already matches the upstream
		// target, the gateway forces no usage option, and no reasoning
		// fixup applies — forward the client's bytes without any JSON
		// round trip.
		if chatReq.ClientModel == modelName && !chatReq.ForceStreamUsage &&
			!chatRequiresToolCallReasoningContent(modelName, chatReq) {
			return chatReq.RawBody, nil
		}
		body, err := patchChatRawBody(chatReq.RawBody, modelName, chatReq.ForceStreamUsage)
		if err != nil {
			return nil, err
		}
		if chatRequiresToolCallReasoningContent(modelName, chatReq) {
			body = types.InjectToolCallReasoningContent(body)
		}
		return body, nil
	}
	// Programmatically built requests (protocol adapters) keep the typed
	// serialization path.
	reqCopy := *chatReq
	reqCopy.Model = modelName
	body, err := json.Marshal(&reqCopy)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}
	if chatRequiresToolCallReasoningContent(modelName, chatReq) {
		body = types.InjectToolCallReasoningContent(body)
	}
	return body, nil
}

// patchChatRawBody rewrites the client's raw chat completion body for the
// upstream: the resolved model name and, when usage metering requires it,
// the streaming include_usage option. Every other field is forwarded as the
// client sent it.
func patchChatRawBody(rawBody []byte, modelName string, addStreamUsage bool) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("parse chat request body: %w", err)
	}
	modelJSON, err := json.Marshal(modelName)
	if err != nil {
		return nil, fmt.Errorf("marshal model name: %w", err)
	}
	payload["model"] = modelJSON
	if addStreamUsage {
		streamOptions := map[string]json.RawMessage{}
		if streamOptionsRaw, ok := payload["stream_options"]; ok {
			if err := json.Unmarshal(streamOptionsRaw, &streamOptions); err != nil || streamOptions == nil {
				streamOptions = map[string]json.RawMessage{}
			}
		}
		streamOptions["include_usage"] = json.RawMessage(`true`)
		payload["stream_options"], err = json.Marshal(streamOptions)
		if err != nil {
			return nil, fmt.Errorf("marshal stream options: %w", err)
		}
	}
	return json.Marshal(payload)
}

// chatRequiresToolCallReasoningContent reports whether the outgoing body
// needs the tool-call reasoning content fixup: DeepSeek-family thinking
// models reject a request that carries tools when an assistant message with
// tool_calls has no reasoning_content field, even on follow-up turns where
// the client never received reasoning output.
func chatRequiresToolCallReasoningContent(modelName string, chatReq *types.ChatCompletionRequest) bool {
	if len(chatReq.Tools) == 0 || chatThinkingDisabled(chatReq) {
		return false
	}
	return types.ModelRequiresToolCallReasoningContent(modelName)
}

// chatThinkingDisabled reports whether the client explicitly disabled
// thinking mode through the top-level `thinking` request field.
func chatThinkingDisabled(chatReq *types.ChatCompletionRequest) bool {
	if len(chatReq.RawJSON) == 0 {
		return false
	}
	var raw struct {
		Thinking *struct {
			Type string `json:"type"`
		} `json:"thinking"`
	}
	if err := json.Unmarshal(chatReq.RawJSON, &raw); err != nil {
		return false
	}
	return raw.Thinking != nil && raw.Thinking.Type == "disabled"
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
