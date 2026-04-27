package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/types"
	commonType "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

type resolvedModelTarget struct {
	Model          *types.Model
	Endpoint       commonType.UpstreamConfig
	TargetReq      commonType.EndpointReq
	Target         string
	Host           string
	ModelName      string
	SessionKeyHash string
	AttemptTargets []chatAttemptTarget
	PrimaryTarget  string
	FallbackTarget string
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

type modelTargetErrorParams struct {
	Status  int
	Code    string
	Message string
	Type    string
	Options modelTargetErrorOptions
}

type endpointTargetResolveInput struct {
	Model     *types.Model
	ModelID   string
	Username  string
	Headers   http.Header
	TargetReq *commonType.EndpointReq
}

type endpointTargetResolveResult struct {
	Endpoint       commonType.UpstreamConfig
	Target         string
	ModelName      string
	SessionKeyHash string
	AttemptTargets []chatAttemptTarget
	PrimaryTarget  string
	FallbackTarget string
}

type chatAttemptTarget struct {
	Target    string
	Endpoint  commonType.UpstreamConfig
	ModelName string
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

func (h *OpenAIHandlerImpl) resolveModelTarget(ctx context.Context, username, modelID string, headers http.Header) (*resolvedModelTarget, error) {
	model, err := h.openaiComponent.GetModelByID(ctx, username, modelID)
	if err != nil {
		return nil, newInternalModelTargetError(err)
	}
	if model == nil {
		return nil, newInvalidRequestModelTargetError("model_not_found", fmt.Sprintf("model '%s' not found", modelID), modelTargetErrorOptions{})
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
		AttemptTargets: make([]chatAttemptTarget, 0, h.chatMaxFallbackAttempts()+1),
	}

	if len(model.SvcName) > 0 {
		resolved.Target, resolved.Host, resolved.ModelName, err = h.resolveCSGHubModelTarget(ctx, model, targetReq)
		if err != nil {
			return nil, err
		}
	} else {
		result, err := h.resolveEndpointModelTarget(ctx, endpointTargetResolveInput{
			Model:     model,
			ModelID:   modelID,
			Username:  username,
			Headers:   headers,
			TargetReq: &targetReq,
		})
		if err != nil {
			return nil, err
		}
		resolved.Endpoint = result.Endpoint
		resolved.Target = result.Target
		resolved.ModelName = result.ModelName
		resolved.SessionKeyHash = result.SessionKeyHash
		resolved.AttemptTargets = result.AttemptTargets
		resolved.PrimaryTarget = result.PrimaryTarget
		resolved.FallbackTarget = result.FallbackTarget
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
	sessionKeyHash := sessionKeyDigest(sessionKey)

	endpoint, err := h.sessionRouter.PickEndpoint(
		input.Model.ID,
		sessionKey,
		input.Model.Upstreams,
		input.Model.RoutingPolicy,
	)
	slog.InfoContext(ctx, "picked endpoint", "session_key", sessionKey, "endpoint", endpoint, "routing_policy", input.Model.RoutingPolicy)
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

	target := input.Model.Endpoint
	selectedEndpoint := commonType.UpstreamConfig{}
	if endpoint.URL != "" {
		selectedEndpoint = endpoint
		target = endpoint.URL
	} else if target != "" {
		selectedEndpoint = component.EndpointByTarget(input.Model.Upstreams, target)
	}

	input.TargetReq.Target = target
	input.TargetReq.Endpoint = target
	input.Model.Endpoint = target
	applyEndpointOverrides(input.Model, selectedEndpoint)

	modelName := resolveEndpointModelName(input.Model.ID, selectedEndpoint)
	attemptTargets := buildChatAttemptTargets(target, input.Model.ID, input.Model.Upstreams, h.chatMaxFallbackAttempts())
	primaryTarget := target
	fallbackTarget := ""
	if len(attemptTargets) > 1 {
		fallbackTarget = attemptTargets[1].Target
	}

	return &endpointTargetResolveResult{
		Endpoint:       selectedEndpoint,
		Target:         target,
		ModelName:      modelName,
		SessionKeyHash: sessionKeyHash,
		AttemptTargets: attemptTargets,
		PrimaryTarget:  primaryTarget,
		FallbackTarget: fallbackTarget,
	}, nil
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
