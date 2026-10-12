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
)

// SetMetricsModelParams bundles inputs for SetMetricsModelTarget so the
// signature stays stable as fields are added.  ModelTarget is nil when
// model resolution fails; in that case ModelID is used as a fallback so
// the error can still be attributed to the requested model.
type SetMetricsModelParams struct {
	C           *gin.Context
	ModelID     string
	ModelTarget *resolvedModelTarget
	IsStream    bool
}

type resolvedModelTarget struct {
	// Model is the resolved logical model. It is MUTABLE across attempts:
	// applyEndpointOverrides/applyChatFallbackTarget write the active
	// upstream's AuthHead/Provider/UpstreamID onto it, and its Endpoint field
	// tracks the active backend URL (the chat proxy derives the upstream path
	// from it via resolveProxyPathFromModelEndpoint).
	Model *types.Model
	// Upstream is the upstream config of the CURRENT attempt: the
	// router-picked one for the primary attempt, replaced wholesale by each
	// fallback (applyChatFallbackTarget). Consumers read AuthHeader,
	// Provider, SupportedEndpoints, IsFullURL, health/circuit state and the
	// per-attempt reporting fields (ID, URL) from it.
	Upstream commonType.UpstreamConfig
	// TargetReq is the legacy CSGHub cluster-deploy endpoint descriptor
	// (cluster ID / svc name / endpoint). It is kept in sync with Target but
	// is NOT part of the proxy path — its only consumer is error reporting
	// (modelTargetError → handleModelTargetError logs it as "targetReq").
	TargetReq commonType.EndpointReq
	// Target is the URL the reverse proxy dials for the current attempt. For
	// the primary attempt it is the planner's BackendURL (see withBackendURL:
	// protocol-routed — native, chat-fallback rewritten, or adapter target);
	// for fallback attempts the protocol-resolved fallback upstream URL
	// (applyChatFallbackTarget). modelTarget.Target is also the URL that
	// must be logged as the routing destination.
	Target          string
	TokenizerTarget string
	Host            string
	ModelName       string
	AttemptTargets  []commonType.UpstreamConfig
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

// ModelErrorCode returns the structured error code so the plan-package
// Planner can categorize the error via errors.As(CodedError) without
// depending on this private type.
func (e *modelTargetError) ModelErrorCode() string {
	return e.APIError.Code
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
	// ClientProtocol is the inference API format the caller is serving.
	// When set, upstream selection prefers candidates that natively support
	// it. Empty for callers that do not do protocol-aware resolution (e.g.
	// multimodal tasks).
	ClientProtocol commonType.ApiFormat
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
	NsUUID             string
	Headers            http.Header
	TargetReq          *commonType.EndpointReq
	RequiredUpstreamID int64
	ClientProtocol     commonType.ApiFormat
}

type endpointTargetResolveResult struct {
	Upstream       commonType.UpstreamConfig
	Target         string
	Host           string
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

func (h *OpenAIHandlerImpl) resolveModelTarget(ctx context.Context, nsUUID, modelID string, headers http.Header) (*resolvedModelTarget, error) {
	return h.resolveModelTargetWithOptions(ctx, nsUUID, modelID, headers, modelTargetResolveOptions{})
}

func (h *OpenAIHandlerImpl) resolveModelTargetWithOptions(ctx context.Context, nsUUID, modelID string, headers http.Header, options modelTargetResolveOptions) (*resolvedModelTarget, error) {
	model, err := h.openaiComponent.GetModelByID(ctx, nsUUID, modelID)
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

	// When the model has upstreams configured (the unified path), prefer the
	// endpoint/upstream routing path. Only fall back to the legacy CSGHub
	// cluster-deploy path for models that have SvcName but no upstreams.
	if len(model.SvcName) > 0 && len(model.Upstreams) == 0 {
		resolved.Target, resolved.Host, resolved.ModelName, err = h.resolveCSGHubModelTarget(ctx, model, targetReq)
		if err != nil {
			return nil, err
		}
	} else {
		result, err := h.resolveEndpointModelTarget(ctx, endpointTargetResolveInput{
			Model:              model,
			ModelID:            modelID,
			NsUUID:             nsUUID,
			Headers:            headers,
			TargetReq:          &targetReq,
			RequiredUpstreamID: options.RequiredUpstreamID,
			ClientProtocol:     options.ClientProtocol,
		})
		if err != nil {
			return nil, err
		}
		resolved.Upstream = result.Upstream
		resolved.Target = result.Target
		resolved.Host = result.Host
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
	resolved.TokenizerTarget = resolved.Target

	return resolved, nil
}

func (h *OpenAIHandlerImpl) resolveCSGHubModelTarget(
	_ context.Context,
	model *types.Model,
	_ commonType.EndpointReq,
) (string, string, string, error) {
	return model.Endpoint, model.InternalModelInfo.Host, model.CSGHubModelID, nil
}

func (h *OpenAIHandlerImpl) resolveEndpointModelTarget(
	ctx context.Context,
	input endpointTargetResolveInput,
) (*endpointTargetResolveResult, error) {
	sessionKey := extractSessionKeyForModel(input.Model, input.Headers, input.NsUUID)
	// normalizedUpstreams filter enable upstream
	normalizedUpstreams := router.NormalizeEnabledUpstreams(input.Model.Upstreams)
	// filterAvailableUpstreams filter healthy statues upstream
	availableUpstreams, availabilityErr := h.filterAvailableUpstreams(ctx, input.Model, normalizedUpstreams)
	if availabilityErr != nil {
		return nil, availabilityErr
	}
	// Keep the full available set on the model, ordered native-capable first,
	// so downstream consumers that iterate in order (capacity admission
	// re-selection) prefer native passthrough while retaining every fallback
	// candidate.
	orderedUpstreams := orderNativeFirst(availableUpstreams, input.ClientProtocol)
	input.Model.Upstreams = orderedUpstreams

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
				Host:           upstreamHostOverride(upstream),
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

	// Prefer upstreams that natively support the client's protocol: when at
	// least one declared/inferable candidate can serve the protocol natively,
	// the router strategy picks from that subset only, so a native
	// passthrough beats a protocol adaptation whenever possible. Fallback
	// candidates keep the same native-first order (full set, all candidates).
	nativeCandidates := preferNativeUpstreams(availableUpstreams, input.ClientProtocol)
	// PickUpstream by router strategy
	upstream, err := h.sessionRouter.PickUpstream(
		input.Model.ID,
		sessionKey,
		nativeCandidates,
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
	attemptTargets := buildChatAttemptTargets(
		upstream,
		orderedUpstreams,
		h.chatMaxFallbackAttempts(),
	)

	return &endpointTargetResolveResult{
		Upstream:       upstream,
		Target:         target,
		Host:           upstreamHostOverride(upstream),
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
	model.UpstreamID = upstream.ID
	// Apply host override from upstream metadata (used for internal csghub deploys
	// that need a specific Host header to reach the correct service). The actual
	// Host header is set later in resolveEndpointModelTarget via upstreamHostOverride().
	if upstreamHostOverride(upstream) != "" {
		model.Endpoint = upstream.URL
	}
}

// upstreamHostOverride extracts the Host header override from upstream metadata.
// This is set by the deploy sync for internal csghub upstreams and is needed
// for k8s multi-tenant routing where the HTTP Host header must match the
// deploy service hostname.
func upstreamHostOverride(upstream commonType.UpstreamConfig) string {
	if upstream.Metadata != nil && upstream.Metadata.InternalModelInfo != nil && upstream.Metadata.InternalModelInfo.Host != "" {
		return upstream.Metadata.InternalModelInfo.Host
	}
	// Backward compat: fall back to legacy top-level HostOverride for old DB rows.
	if upstream.Metadata != nil && upstream.Metadata.HostOverride != "" {
		return upstream.Metadata.HostOverride
	}
	return ""
}

// nativeCapableForClient reports whether an upstream can serve the client
// protocol natively: its API-format declaration (or URL/metadata inference)
// includes the protocol, or its format is unknown — unknown-format upstreams
// keep the legacy behavior of a native passthrough for whatever protocol the
// client speaks.
func nativeCapableForClient(u commonType.UpstreamConfig, clientProtocol commonType.ApiFormat) bool {
	return u.SupportsProtocol(clientProtocol) || !u.HasProtocolDeclaration()
}

// orderNativeFirst partitions the candidate list into native-capable
// upstreams first, preserving relative order within each group. The full
// candidate set is kept (unlike preferNativeUpstreams, which narrows), so
// downstream consumers that iterate in order — capacity admission
// re-selection, fallback target ordering — prefer native passthrough but can
// still fall back to adapter-served upstreams.
func orderNativeFirst(upstreams []commonType.UpstreamConfig, clientProtocol commonType.ApiFormat) []commonType.UpstreamConfig {
	if strings.TrimSpace(string(clientProtocol)) == "" || len(upstreams) <= 1 {
		return upstreams
	}
	native := make([]commonType.UpstreamConfig, 0, len(upstreams))
	rest := make([]commonType.UpstreamConfig, 0, len(upstreams))
	for _, u := range upstreams {
		if nativeCapableForClient(u, clientProtocol) {
			native = append(native, u)
		} else {
			rest = append(rest, u)
		}
	}
	if len(native) == 0 || len(rest) == 0 {
		return upstreams
	}
	return append(native, rest...)
}

// preferNativeUpstreams narrows the candidate list to the upstreams that can
// serve the client protocol natively. When no candidate natively supports the
// protocol (or the client protocol is not protocol-aware routing), the
// original list is returned unchanged and the legacy adapter routing decides.
func preferNativeUpstreams(upstreams []commonType.UpstreamConfig, clientProtocol commonType.ApiFormat) []commonType.UpstreamConfig {
	if strings.TrimSpace(string(clientProtocol)) == "" || len(upstreams) <= 1 {
		return upstreams
	}
	native := make([]commonType.UpstreamConfig, 0, len(upstreams))
	rest := make([]commonType.UpstreamConfig, 0, len(upstreams))
	for _, u := range upstreams {
		if nativeCapableForClient(u, clientProtocol) {
			native = append(native, u)
		} else {
			rest = append(rest, u)
		}
	}
	if len(native) == 0 || len(rest) == 0 {
		return upstreams
	}
	return native
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
