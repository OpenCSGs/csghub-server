package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component/router"
	"opencsg.com/csghub-server/aigateway/types"
	commonType "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

type resolvedModelTarget struct {
	Model          *types.Model
	Upstream       commonType.UpstreamConfig
	TargetReq      commonType.EndpointReq
	Target         string
	Host           string
	ModelName      string
	AttemptTargets []commonType.UpstreamConfig
}

const maxSessionKeyLength = 256

type modelTargetError struct {
	Status    int
	APIError  types.Error
	Cause     error
	Model     *types.Model
	TargetReq commonType.EndpointReq
	Target    string
	Host      string
}

func (e *modelTargetError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.APIError.Message
}

type modelTargetErrorOptions struct {
	Cause     error
	Model     *types.Model
	TargetReq commonType.EndpointReq
	Target    string
	Host      string
}

type modelTargetResolveOptions struct {
	RequiredUpstreamID int64
}

type modelTargetErrorParams struct {
	Status  int
	Code    string
	Message string
	Type    string
	Options modelTargetErrorOptions
}

type endpointTargetResolveInput struct {
	Model              *types.Model
	ModelID            string
	Username           string
	Headers            http.Header
	TargetReq          *commonType.EndpointReq
	RequiredUpstreamID int64
}

type endpointTargetResolveResult struct {
	Upstream       commonType.UpstreamConfig
	Target         string
	ModelName      string
	AttemptTargets []commonType.UpstreamConfig
}

func (h *OpenAIHandlerImpl) chatMaxFallbackAttempts() int {
	if h == nil || h.config == nil {
		return defaultChatMaxFallbackAttempts
	}
	return normalizeChatMaxFallbackAttempts(h.config.AIGateway.ChatMaxFallbackAttempts)
}

func newModelTargetError(params modelTargetErrorParams) *modelTargetError {
	return &modelTargetError{
		Status: params.Status,
		APIError: types.Error{
			Code:    params.Code,
			Message: params.Message,
			Type:    params.Type,
		},
		Cause:     params.Options.Cause,
		Model:     params.Options.Model,
		TargetReq: params.Options.TargetReq,
		Target:    params.Options.Target,
		Host:      params.Options.Host,
	}
}

func newInternalModelTargetError(err error) *modelTargetError {
	return newModelTargetError(modelTargetErrorParams{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: err.Error(),
		Type:    "internal_error",
		Options: modelTargetErrorOptions{Cause: err},
	})
}

func newInvalidRequestModelTargetError(code, message string, options modelTargetErrorOptions) *modelTargetError {
	return newModelTargetError(modelTargetErrorParams{
		Status:  http.StatusBadRequest,
		Code:    code,
		Message: message,
		Type:    "invalid_request_error",
		Options: options,
	})
}

func newServerModelTargetError(code, message string, options modelTargetErrorOptions) *modelTargetError {
	return newModelTargetError(modelTargetErrorParams{
		Status:  http.StatusInternalServerError,
		Code:    code,
		Message: message,
		Type:    "server_error",
		Options: options,
	})
}

func (h *OpenAIHandlerImpl) resolveModelTarget(ctx context.Context, username, modelID string, headers http.Header) (*resolvedModelTarget, error) {
	return h.resolveModelTargetWithOptions(ctx, username, modelID, headers, modelTargetResolveOptions{})
}

func (h *OpenAIHandlerImpl) resolveModelTargetWithOptions(ctx context.Context, username, modelID string, headers http.Header, options modelTargetResolveOptions) (*resolvedModelTarget, error) {
	model, err := h.openaiComponent.GetModelByID(ctx, username, modelID)
	if err != nil {
		return nil, newInternalModelTargetError(err)
	}
	if model == nil {
		return nil, newInvalidRequestModelTargetError("model_not_found", fmt.Sprintf("model '%s' not found", modelID), modelTargetErrorOptions{})
	}
	requiresSKUPrice, hasConfiguredSKUPrice := modelSKUPriceStatus(model)
	if requiresSKUPrice && !hasConfiguredSKUPrice {
		return nil, newServerModelTargetError(
			"model_price_not_configured",
			"target model has no configured SKU price",
			modelTargetErrorOptions{Model: model},
		)
	}

	targetReq := commonType.EndpointReq{
		ClusterID: model.ClusterID,
		Target:    model.Endpoint,
		Host:      "",
		Endpoint:  model.Endpoint,
		SvcName:   model.SvcName,
	}

	resolved := &resolvedModelTarget{
		Model:          model,
		TargetReq:      targetReq,
		AttemptTargets: make([]commonType.UpstreamConfig, 0, h.chatMaxFallbackAttempts()+1),
	}

	if len(model.SvcName) > 0 {
		resolved.Target, resolved.Host, resolved.ModelName, err = h.resolveCSGHubModelTarget(ctx, model, targetReq)
		if err != nil {
			return nil, err
		}
	} else {
		result, err := h.resolveEndpointModelTarget(ctx, endpointTargetResolveInput{
			Model:              model,
			ModelID:            modelID,
			Username:           username,
			Headers:            headers,
			TargetReq:          &targetReq,
			RequiredUpstreamID: options.RequiredUpstreamID,
		})
		if err != nil {
			return nil, err
		}
		resolved.Upstream = result.Upstream
		resolved.Target = result.Target
		resolved.ModelName = result.ModelName
		resolved.AttemptTargets = result.AttemptTargets
	}

	if len(resolved.Target) < 1 {
		return nil, newInvalidRequestModelTargetError(
			"model_not_running",
			fmt.Sprintf("model '%s' not running", modelID),
			modelTargetErrorOptions{
				Model:     model,
				TargetReq: resolved.TargetReq,
				Target:    resolved.Target,
				Host:      resolved.Host,
			},
		)
	}

	return resolved, nil
}

func (h *OpenAIHandlerImpl) resolveCSGHubModelTarget(
	ctx context.Context,
	model *types.Model,
	targetReq commonType.EndpointReq,
) (string, string, string, error) {
	cluster, err := h.clusterComp.GetClusterByID(ctx, targetReq.ClusterID)
	if err != nil {
		return "", "", "", newInvalidRequestModelTargetError(
			"cluster_not_found",
			fmt.Sprintf("cluster '%s' not found", model.ClusterID),
			modelTargetErrorOptions{
				Cause:     err,
				Model:     model,
				TargetReq: targetReq,
			},
		)
	}
	target, host, _ := common.ExtractDeployTargetAndHost(ctx, cluster, targetReq)
	return target, host, model.CSGHubModelID, nil
}

func (h *OpenAIHandlerImpl) resolveEndpointModelTarget(
	ctx context.Context,
	input endpointTargetResolveInput,
) (*endpointTargetResolveResult, error) {
	sessionKey := extractSessionKeyForModel(input.Model, input.Headers, input.Username)
	// normalizedUpstreams filter enable upstream
	normalizedUpstreams := router.NormalizeEnabledUpstreams(input.Model.Upstreams)
	// filterAvailableUpstreams filter healthy statues upstream
	availableUpstreams, availabilityErr := h.filterAvailableUpstreams(ctx, input.Model, normalizedUpstreams)
	if availabilityErr != nil {
		return nil, availabilityErr
	}
	input.Model.Upstreams = availableUpstreams

	if input.RequiredUpstreamID != 0 {
		for _, upstream := range input.Model.Upstreams {
			if upstream.ID != input.RequiredUpstreamID {
				continue
			}
			target := upstream.URL
			input.TargetReq.Target = target
			input.TargetReq.Endpoint = target
			input.Model.Endpoint = target
			applyEndpointOverrides(input.Model, upstream)
			modelName := resolveEndpointModelName(input.Model.ID, upstream)
			return &endpointTargetResolveResult{
				Upstream:       upstream,
				Target:         target,
				ModelName:      modelName,
				AttemptTargets: []commonType.UpstreamConfig{upstream},
			}, nil
		}
		return nil, newInvalidRequestModelTargetError(
			"required_upstream_unavailable",
			fmt.Sprintf("required upstream %d for model '%s' is unavailable", input.RequiredUpstreamID, input.ModelID),
			modelTargetErrorOptions{
				Model:     input.Model,
				TargetReq: *input.TargetReq,
			},
		)
	}

	// PickUpstream by router strategy
	upstream, err := h.sessionRouter.PickUpstream(
		input.Model.ID,
		sessionKey,
		input.Model.Upstreams,
		input.Model.RoutingPolicy,
	)
	slog.InfoContext(
		ctx,
		"picked upstream",
		"session_key",
		sessionKey,
		"upstream",
		upstream.URL,
		"upstream_provider",
		upstream.Provider,
		"upstream_weight",
		upstream.Weight,
		"routing_policy",
		input.Model.RoutingPolicy,
	)
	if err != nil && input.Model.Endpoint == "" {
		return nil, newInvalidRequestModelTargetError(
			"model_not_running",
			fmt.Sprintf("model '%s' not running", input.ModelID),
			modelTargetErrorOptions{
				Cause:     err,
				Model:     input.Model,
				TargetReq: *input.TargetReq,
			},
		)
	}

	target := upstream.URL
	input.TargetReq.Target = target
	input.TargetReq.Endpoint = target
	input.Model.Endpoint = target
	// orverride models's AuthHead, Provider with upstream
	applyEndpointOverrides(input.Model, upstream)
	modelName := resolveEndpointModelName(input.Model.ID, upstream)
	attemptTargets := buildChatAttemptTargets(upstream, availableUpstreams, h.chatMaxFallbackAttempts())

	return &endpointTargetResolveResult{
		Upstream:       upstream,
		Target:         target,
		ModelName:      modelName,
		AttemptTargets: attemptTargets,
	}, nil
}

func (h *OpenAIHandlerImpl) filterAvailableUpstreams(
	ctx context.Context,
	model *types.Model,
	upstreams []commonType.UpstreamConfig,
) ([]commonType.UpstreamConfig, error) {
	if len(upstreams) == 0 || model == nil {
		return upstreams, nil
	}

	filtered := make([]commonType.UpstreamConfig, 0, len(upstreams))
	for _, u := range upstreams {
		if types.IsUpstreamUnhealthy(u) {
			continue
		} else if types.IsUpstreamCircuitOpen(u) && h.IsCacheUpstreamCircuitOpen(ctx, u) {
			continue
		}
		filtered = append(filtered, u)
	}
	if len(filtered) == 0 {
		return nil, newModelTargetError(modelTargetErrorParams{
			Status:  http.StatusServiceUnavailable,
			Code:    "model_unavailable",
			Message: fmt.Sprintf("model '%s' has no available upstream endpoint", model.ID),
			Type:    "service_unavailable_error",
			Options: modelTargetErrorOptions{Model: model},
		})
	}
	return filtered, nil
}

// IsCacheUpstreamCircuitOpen returns true only when the runtime circuit cache
// explicitly reports this upstream as open.
//
// Design note:
//   - If the availability manager is not initialized, return false and allow the
//     proxy request to proceed.
//   - If reading the runtime circuit state fails, return false and allow the
//     proxy request to proceed.
//   - If the runtime cache reports a non-open state, return false and allow the
//     proxy request to proceed even when the persisted upstream state says open.
//
// In short, we intentionally keep circuit filtering permissive here. We only
// block an upstream when the runtime cache confirms it is truly open, so users
// still have the best chance to reach a proxy target instead of being rejected
// too early.
func (h *OpenAIHandlerImpl) IsCacheUpstreamCircuitOpen(ctx context.Context, upstream commonType.UpstreamConfig) bool {
	if h.availabilityManager == nil {
		return false
	}

	rtState, err := h.availabilityManager.GetCircuitState(ctx, upstream.ID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get circuit state from cache",
			"error", err)
		return false
	}
	if rtState.CircuitState == types.CircuitStateOpen {
		return true
	}
	slog.WarnContext(ctx, "circuit state from cache differs from DB, allowing upstream",
		"upstream_id", upstream.ID, "db_state", types.CircuitStateOpen, "rt_state", rtState.CircuitState)
	return false
}

func resolveEndpointModelName(defaultModelName string, upstream commonType.UpstreamConfig) string {
	if modelName := strings.TrimSpace(upstream.ModelName); modelName != "" {
		return modelName
	}
	return defaultModelName
}

func applyEndpointOverrides(model *types.Model, upstream commonType.UpstreamConfig) {
	if model == nil {
		return
	}
	if strings.TrimSpace(upstream.AuthHeader) != "" {
		model.AuthHead = upstream.AuthHeader
	}
	model.Provider = upstream.Provider
}

func extractSessionKeyForModel(model *types.Model, headers http.Header, fallbackSessionKey string) string {
	headerCandidates := []string{
		strings.TrimSpace(model.RoutingPolicy.SessionHeader),
		"X-Session-ID",
		"X-Conversation-ID",
	}
	if headers != nil {
		for _, headerName := range headerCandidates {
			if headerName == "" {
				continue
			}
			if value := strings.TrimSpace(headers.Get(headerName)); value != "" {
				if len(value) > maxSessionKeyLength {
					return value[:maxSessionKeyLength]
				}
				return value
			}
		}
	}
	fallbackSessionKey = strings.TrimSpace(fallbackSessionKey)
	if fallbackSessionKey != "" {
		if len(fallbackSessionKey) > maxSessionKeyLength {
			return fallbackSessionKey[:maxSessionKeyLength]
		}
		return fallbackSessionKey
	}
	return ""
}

func handleModelTargetError(c *gin.Context, ctx context.Context, modelID, logMessage string, err error) {
	var targetErr *modelTargetError
	if !errors.As(err, &targetErr) {
		slog.ErrorContext(ctx, logMessage, slog.String("model_id", modelID), slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code:    "internal_error",
			Message: err.Error(),
			Type:    "internal_error",
		}})
		return
	}

	switch targetErr.APIError.Code {
	case "internal_error":
		slog.ErrorContext(ctx, "failed to get model by id", slog.String("model_id", modelID), slog.Any("error", targetErr.Cause))
	case "model_not_running":
		slog.ErrorContext(ctx, logMessage, slog.Any("model", targetErr.Model), slog.Any("targetReq", targetErr.TargetReq),
			slog.String("model_id", modelID), slog.String("target", targetErr.Target), slog.String("host", targetErr.Host))
	}

	c.JSON(targetErr.Status, gin.H{"error": targetErr.APIError})
}
