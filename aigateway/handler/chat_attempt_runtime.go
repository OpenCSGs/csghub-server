package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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
	updateChatAttemptRuntime(tokenCounter, logCapture, modelTarget)
}
