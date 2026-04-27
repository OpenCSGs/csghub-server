package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2image"
	"opencsg.com/csghub-server/aigateway/http/response/wrapper"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	commonType "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/trace"
	apicomp "opencsg.com/csghub-server/component"
)

// OpenAIHandler defines the interface for handling OpenAI compatible APIs
type OpenAIHandler interface {
	// List available models
	ListModels(c *gin.Context)
	// Get model details
	GetModel(c *gin.Context)
	// Chat with backend model
	Chat(c *gin.Context)
	Embedding(c *gin.Context)
	// Generate image from text
	GenerateImage(c *gin.Context)
	// Transcribe audio to text
	Transcription(c *gin.Context)
	SetChatAttemptFailureReporter(reporter ChatAttemptFailureReporter)
}

func NewOpenAIHandlerFromConfig(config *config.Config) (OpenAIHandler, error) {
	modelService, err := component.NewOpenAIComponentFromConfig(config)
	if err != nil {
		return nil, err
	}
	repoComp, err := apicomp.NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	var modSvcClient rpc.ModerationSvcClient
	var cacheClient cache.RedisClient
	if config.SensitiveCheck.Enable {
		modSvcClient = rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", config.Moderation.Host, config.Moderation.Port))
		cacheClient, err = cache.NewCache(context.Background(), cache.RedisConfig{
			Addr:     config.Redis.Endpoint,
			Username: config.Redis.User,
			Password: config.Redis.Password,
		})
		if err != nil {
			return nil, err
		}
	}
	modComponent := component.NewModerationImplWithClient(config, modSvcClient, cacheClient)
	clusterComp, err := apicomp.NewClusterComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster component, error: %w", err)
	}
	storage, _ := component.NewStorage(config)
	whitelistRule := database.NewRepositoryFileCheckRuleStore()
	return newOpenAIHandler(modelService, repoComp, modComponent, clusterComp, token.NewCounterFactory(), text2image.NewRegistry(), config, storage, whitelistRule), nil
}

func newOpenAIHandler(
	modelService component.OpenAIComponent,
	repoComp apicomp.RepoComponent,
	modComponent component.Moderation,
	clusterComp apicomp.ClusterComponent,
	tokenCounterFactory token.CounterFactory,
	t2iRegistry *text2image.Registry,
	config *config.Config,
	storage types.Storage,
	whitelistRule database.RepositoryFileCheckRuleStore,
) *OpenAIHandlerImpl {
	return &OpenAIHandlerImpl{
		openaiComponent:            modelService,
		repoComp:                   repoComp,
		modComponent:               modComponent,
		clusterComp:                clusterComp,
		tokenCounterFactory:        tokenCounterFactory,
		t2iRegistry:                t2iRegistry,
		config:                     config,
		storage:                    storage,
		whitelistRule:              whitelistRule,
		sensitivePolicy:            component.NewSensitivePolicy(modComponent, whitelistRule),
		llmLogPublisher:            component.NewLLMLogPublisher(),
		sessionRouter:              component.NewSessionRouter(),
		chatAttemptFailureReporter: noopChatAttemptFailureReporter{},
	}
}

// handleInsufficientBalance handles the insufficient balance error response
// for both stream and non-stream requests
func (h *OpenAIHandlerImpl) handleInsufficientBalance(c *gin.Context, isStream bool, nsUUID, modelID string, err error) {
	// Check if the error is the standard insufficient balance error
	if !errors.Is(err, errorx.ErrInsufficientBalance) {
		// If it's a different error, log and return generic error
		slog.ErrorContext(c.Request.Context(), "balance check failed", slog.Any("ns_uuid", nsUUID),
			slog.Any("model", modelID), slog.Any("error", err))
		httpbase.ServerError(c, err)
		return
	}

	slog.WarnContext(c.Request.Context(), "insufficient balance for request",
		slog.Any("ns_uuid", nsUUID), slog.Any("model", modelID))

	if isStream {
		// For stream requests, write error chunk
		errorChunk := generateInsufficientBalanceResp(h.config.Frontend.URL)
		errorChunkJson, _ := json.Marshal(errorChunk)
		_, writeErr := c.Writer.Write([]byte("data: " + string(errorChunkJson) + "\n\ndata: [DONE]\n\n"))
		if writeErr != nil {
			slog.Error("failed to write insufficient balance error to stream", "error", writeErr)
		}
		c.Writer.Flush()
	} else {
		httpbase.ForbiddenError(c, err)
	}
}

func (h *OpenAIHandlerImpl) handleUsageLimitExceeded(c *gin.Context, isStream bool, username, modelID string, err error) {
	if !component.IsUsageLimitExceeded(err) {
		slog.ErrorContext(c.Request.Context(), "usage limit check failed",
			"user", username, "model", modelID, "error", err)
		httpbase.ServerError(c, err)
		return
	}

	slog.WarnContext(c.Request.Context(), "usage limit exceeded for request",
		"user", username, "model", modelID)

	payload := gin.H{
		"error": gin.H{
			"code":    "rate_limit_exceeded",
			"message": "Usage quota exceeded for current window",
			"type":    "rate_limit_error",
		},
	}
	if isStream {
		errorChunkJSON, _ := json.Marshal(payload)
		_, writeErr := c.Writer.Write([]byte("data: " + string(errorChunkJSON) + "\n\ndata: [DONE]\n\n"))
		if writeErr != nil {
			slog.Error("failed to write rate limit error to stream", "error", writeErr)
		}
		c.Writer.Flush()
		return
	}
	c.JSON(http.StatusTooManyRequests, payload)
}

func (h *OpenAIHandlerImpl) handleProxyError(c *gin.Context, isStream bool, username, modelID string, err error) {
	if component.IsUsageLimitExceeded(err) {
		h.handleUsageLimitExceeded(c, isStream, username, modelID, err)
		return
	}
	slog.ErrorContext(c.Request.Context(), "failed to create reverse proxy",
		slog.Any("error", err))
	httpbase.ServerError(c, err)
}

// OpenAIHandlerImpl implements the OpenAIHandler interface
type OpenAIHandlerImpl struct {
	openaiComponent            component.OpenAIComponent
	repoComp                   apicomp.RepoComponent
	modComponent               component.Moderation
	clusterComp                apicomp.ClusterComponent
	tokenCounterFactory        token.CounterFactory
	t2iRegistry                *text2image.Registry
	config                     *config.Config
	storage                    types.Storage
	whitelistRule              database.RepositoryFileCheckRuleStore
	sensitivePolicy            component.SensitivePolicy
	llmLogPublisher            component.LLMLogPublisher
	sessionRouter              component.SessionRouter
	chatAttemptFailureReporter ChatAttemptFailureReporter
}

// ListModels godoc
// @Summary      List available models
// @Description  Returns a list of available models, supports fuzzy search by model_id query parameter and filtering by source and task
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        model_id query string false "Model ID for fuzzy search"
// @Param        source query string false "Filter by source (csghub for CSGHub models, external for external models)" Enums(csghub, external)
// @Param        task query string false "Filter by task (e.g., text-generation, text-to-image)"
// @Param        per query int false "Models per page (default 20, max 100)"
// @Param        page query int false "Page number (1-based, default 1)"
// @Success      200  {object}  types.ModelList "OK"
// @Failure      400  {object}  error "Invalid source parameter"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/models [get]
func (h *OpenAIHandlerImpl) ListModels(c *gin.Context) {
	currentUser := httpbase.GetCurrentUser(c)

	// Validate source parameter
	source := strings.TrimSpace(c.Query("source"))
	if source != "" {
		sourceLower := strings.ToLower(source)
		if sourceLower != string(types.ModelSourceCSGHub) && sourceLower != string(types.ModelSourceExternal) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": types.Error{
					Code:    "invalid_request_error",
					Message: fmt.Sprintf("Invalid source parameter. Must be '%s' or '%s'", types.ModelSourceCSGHub, types.ModelSourceExternal),
					Type:    "invalid_request_error",
				}})
			return
		}
	}

	resp, err := h.openaiComponent.ListModels(c.Request.Context(), currentUser, types.ListModelsReq{
		ModelID: c.Query("model_id"),
		Source:  source,
		Task:    c.Query("task"),
		Per:     c.Query("per"),
		Page:    c.Query("page"),
	})
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to get available models", "error", err.Error(), "current_user", currentUser)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": types.Error{
				Code:    "internal_server_error",
				Message: "Failed to retrieve models",
				Type:    "server_error",
			}})
		return
	}
	c.PureJSON(http.StatusOK, resp)
}

// GetModel godoc
// @Security     ApiKey
// @Summary      Get model details
// @Description  Returns information about a specific model
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        model path string true "Model ID"
// @Success      200  {object}  types.Model "OK"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/models/{model} [get]
func (h *OpenAIHandlerImpl) GetModel(c *gin.Context) {
	username := httpbase.GetCurrentUser(c)
	modelID := c.Param("model")
	modelID = strings.TrimPrefix(modelID, "/")
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": types.Error{
				Code:    "model_not_found",
				Message: "model id can not be empty",
				Type:    "invalid_request_error",
			}})
		return
	}

	model, err := h.openaiComponent.GetModelByID(c.Request.Context(), username, modelID)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Errorf("failed to get model by id '%s',error:%w", modelID, err).Error())
		return
	}
	if model == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": types.Error{
				Code:    "model_not_found",
				Message: fmt.Sprintf("model '%s' not found", modelID),
				Type:    "invalid_request_error",
			}})
		return
	}

	if model.FormatModelID != "" {
		model.ID = model.FormatModelID
	}

	c.PureJSON(http.StatusOK, model)
}

var _ openai.ChatCompletion
var _ openai.ChatCompletionChunk

// Chat godoc
// @Security     ApiKey
// @Summary      Chat with backend model
// @Description  Sends a chat completion request to the backend model and returns the response
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request body ChatCompletionRequest true "Chat completion request"
// @Success      200  {object}  openai.ChatCompletion "OK"
// @Success      200  {object}  openai.ChatCompletionChunk "OK"
// @Failure      400  {object}  error "Bad request"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/chat/completions [post]
func (h *OpenAIHandlerImpl) Chat(c *gin.Context) {
	/*
		1.parse request body of ChatCompletionRequest
		2.get model id from request body
		3.find running model endpoint by model id
		4.proxy request to running model endpoint
	*/
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)
	chatReq := &ChatCompletionRequest{}
	if err := c.BindJSON(chatReq); err != nil {
		slog.ErrorContext(c.Request.Context(), "invalid chat completion request body", slog.Any("error", err))
		c.String(http.StatusBadRequest, fmt.Errorf("invalid chat completion request body:%w", err).Error())
		return
	}
	modelID := chatReq.Model
	modelTarget, err := h.resolveModelTarget(c.Request.Context(), username, modelID, c.Request.Header)
	if err != nil {
		handleModelTargetError(c, c.Request.Context(), modelID, "failed to get model target address", err)
		return
	}

	chatReq.Model = modelTarget.ModelName
	if chatReq.Stream {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		if !strings.Contains(modelTarget.Model.ImageID, "vllm-cpu") {
			chatReq.StreamOptions = &StreamOptions{
				IncludeUsage: true,
			}
		}
	}

	// Check balance before processing request
	if !modelTarget.Model.SkipBalance() {
		if err := h.openaiComponent.CheckBalance(c.Request.Context(), nsUUID); err != nil {
			h.handleInsufficientBalance(c, chatReq.Stream, nsUUID, modelID, err)
			return
		}
	}
	var modComponent component.Moderation = nil
	isCheck, result, err := h.sensitivePolicy.CheckChatSensitive(c.Request.Context(), modelTarget.Model, chatReq.Messages, nsUUID, chatReq.Stream)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to check sensitive",
			slog.String("model_id", modelID),
			slog.String("username", username),
			slog.Any("error", err))
	}
	if isCheck {
		modComponent = h.modComponent
		if result != nil && result.IsSensitive {
			handleSensitiveResponse(c, chatReq.Stream, result)
			return
		}
	}

	slog.InfoContext(c.Request.Context(), "proxy chat request to model target",
		slog.Any("target", modelTarget.Target),
		slog.Any("host", modelTarget.Host),
		slog.Any("ns_uuid", nsUUID),
		slog.Any("model_name", modelTarget.ModelName))

	chatCtx := h.setupChatContext(
		c.Request.Context(),
		modelTarget,
		chatReq,
		modComponent,
		c.Writer,
		trace.GetTraceIDInGinContext(c),
		nsUUID,
	)
	defer chatCtx.responseWriter.ClearBuffer()

	chatCtx.tokenCounter.AppendPrompts(chatReq.Messages)

	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(c.Request.Context(), "invalid auth head",
			slog.String("model", modelTarget.ModelName),
			slog.Any("error", err))
	}

	primaryWriter, proxyErr := h.executeChatProxyAttempt(c, chatCtx.responseWriter, modelTarget, nsUUID, chatReq)
	if proxyErr != nil {
		h.handleProxyError(c, chatReq.Stream, username, modelID, proxyErr)
		return
	}

	if err := h.executeChatWithFallback(c, chatCtx, modelTarget, nsUUID, chatReq, primaryWriter, username, modelID); err != nil {
		h.handleProxyError(c, chatReq.Stream, username, modelID, err)
		return
	}

	h.runChatPostProcessAsync(c.Request.Context(), chatPostProcessInput{
		NSUUID:       nsUUID,
		ApiKey:       apikey,
		Model:        modelTarget.Model,
		TokenCounter: chatCtx.tokenCounter,
		LogCapture:   chatCtx.logCapture,
	})
}

type chatPostProcessInput struct {
	NSUUID       string
	ApiKey       string
	Model        *types.Model
	TokenCounter token.Counter
	LogCapture   component.LLMLogRecorder
}

type chatContext struct {
	tokenCounter   token.ChatTokenCounter
	logCapture     component.LLMLogRecorder
	responseWriter CommonResponseWriter
}

func (h *OpenAIHandlerImpl) setupChatContext(
	ctx context.Context,
	modelTarget *resolvedModelTarget,
	chatReq *ChatCompletionRequest,
	modComponent component.Moderation,
	ginWriter gin.ResponseWriter,
	traceID string,
	nsUUID string,
) *chatContext {
	tokenCounter := h.tokenCounterFactory.NewChat(token.CreateParam{
		Endpoint: modelTarget.Target,
		Host:     modelTarget.Host,
		Model:    modelTarget.ModelName,
		ImageID:  modelTarget.Model.ImageID,
		Provider: modelTarget.Model.Provider,
	})

	logCapture, err := component.NewLLMLogRecorder(
		traceID,
		modelTarget.ModelName,
		nsUUID,
		commonType.LLMLogRequest{
			Messages: chatReq.Messages,
			Tools:    chatReq.Tools,
			Stream:   chatReq.Stream,
		},
		map[string]any{
			"source":   "aigateway",
			"api":      "/v1/chat/completions",
			"stream":   chatReq.Stream,
			"provider": modelTarget.Model.Provider,
			"svc_name": modelTarget.Model.SvcName,
		},
	)
	if err != nil {
		slog.WarnContext(ctx, "failed to initialize llmlog training capture", slog.Any("error", err))
	}

	responseWriter := NewResponseWriterWrapper(ginWriter, chatReq.Stream, modComponent, tokenCounter, logCapture)

	return &chatContext{
		tokenCounter:   tokenCounter,
		logCapture:     logCapture,
		responseWriter: responseWriter,
	}
}

func (h *OpenAIHandlerImpl) executeChatWithFallback(
	c *gin.Context,
	chatCtx *chatContext,
	modelTarget *resolvedModelTarget,
	userUUID string,
	chatReq *ChatCompletionRequest,
	primaryWriter *chatRetryResponseWriter,
	username string,
	modelID string,
) error {
	primaryStatusCode := primaryWriter.StatusCode()
	primaryStreamStarted := primaryWriter.StreamStarted()
	primaryRetryable := shouldRetryChatAttempt(primaryStatusCode, primaryStreamStarted)

	if shouldReportChatAttemptFailure(primaryStatusCode) {
		h.reportChatAttemptFailure(c.Request.Context(), ChatAttemptFailureEvent{
			Phase:          chatAttemptPhasePrimary,
			ModelID:        modelTarget.Model.ID,
			ModelName:      modelTarget.ModelName,
			Provider:       modelTarget.Model.Provider,
			Endpoint:       modelTarget.Endpoint.URL,
			Target:         modelTarget.Target,
			SessionKeyHash: modelTarget.SessionKeyHash,
			StatusCode:     primaryStatusCode,
			Retryable:      primaryRetryable,
		})
	}

	hasFallbacks := len(modelTarget.AttemptTargets) > 1
	if !primaryRetryable || !hasFallbacks {
		if replayErr := primaryWriter.ReplayBufferedResponse(); replayErr != nil {
			slog.WarnContext(c.Request.Context(), "failed to replay buffered response", slog.Any("error", replayErr))
		}
		return nil
	}

	slog.InfoContext(c.Request.Context(), "retry chat request with fallback endpoint",
		slog.String("model_id", modelID),
		slog.String("session_key_hash", modelTarget.SessionKeyHash),
		slog.String("primary_endpoint", modelTarget.PrimaryTarget),
		slog.String("next_fallback_endpoint", modelTarget.FallbackTarget),
		slog.Int("available_fallback_attempts", len(modelTarget.AttemptTargets)-1),
		slog.String("retry_reason", chatRetryReason(primaryStatusCode)),
		slog.Int("status_code", primaryStatusCode))

	retryErr := h.retryChatWithFallback(c, chatCtx.responseWriter, modelTarget, userUUID, chatReq, chatCtx.tokenCounter, chatCtx.logCapture)
	if retryErr != nil {
		if component.IsUsageLimitExceeded(retryErr) {
			return retryErr
		}
		slog.ErrorContext(c.Request.Context(), "fallback chat retry failed", slog.Any("error", retryErr))
		if replayErr := primaryWriter.ReplayBufferedResponse(); replayErr != nil {
			slog.WarnContext(c.Request.Context(), "failed to replay buffered fallback response after retry error", slog.Any("error", replayErr))
		}
	}
	return nil
}

func (h *OpenAIHandlerImpl) runChatPostProcessAsync(ctx context.Context, input chatPostProcessInput) {
	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()

		if err := h.openaiComponent.CommitUsageLimit(usageCtx, input.NSUUID, input.Model, input.TokenCounter); err != nil {
			slog.ErrorContext(usageCtx, "failed to commit usage limit", slog.Any("error", err))
		}

		if err := h.openaiComponent.RecordUsage(usageCtx, input.NSUUID, input.Model, input.TokenCounter, input.ApiKey); err != nil {
			slog.ErrorContext(usageCtx, "failed to record token usage", slog.Any("error", err))
		}

		if h.config.AIGateway.EnableLLMLog && input.LogCapture != nil && h.llmLogPublisher != nil {
			record, recordErr := input.LogCapture.Record()
			if recordErr != nil {
				slog.ErrorContext(usageCtx, "failed to build llmlog training record", slog.Any("error", recordErr))
				return
			}
			payload, marshalErr := json.Marshal(record)
			if marshalErr != nil {
				slog.ErrorContext(usageCtx, "failed to marshal llmlog training record", slog.Any("error", marshalErr))
				return
			}
			if publishErr := h.llmLogPublisher.PublishTrainingLog(payload); publishErr != nil {
				slog.ErrorContext(usageCtx, "failed to publish llmlog training record", slog.Any("error", publishErr))
			}
		}
	}()
}

func (h *OpenAIHandlerImpl) executeChatProxyAttempt(c *gin.Context, w CommonResponseWriter, modelTarget *resolvedModelTarget, userUUID string, chatReq *ChatCompletionRequest) (*chatRetryResponseWriter, error) {
	if err := h.openaiComponent.CheckUsageLimit(c.Request.Context(), userUUID, modelTarget.Model, modelTarget.Target); err != nil {
		return nil, err
	}
	body, err := marshalChatRequestBody(chatReq, modelTarget.ModelName)
	if err != nil {
		return nil, err
	}
	proxyToAPI := resolveProxyPathFromModelEndpoint(modelTarget.Model.Endpoint, modelTarget.ModelName)
	rp, err := proxy.NewReverseProxy(modelTarget.Target)
	if err != nil {
		return nil, err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	retryWriter := newChatRetryResponseWriter(w)
	rp.ServeHTTP(retryWriter, c.Request, proxyToAPI, modelTarget.Host)
	return retryWriter, nil
}

func (h *OpenAIHandlerImpl) retryChatWithFallback(c *gin.Context, w CommonResponseWriter, modelTarget *resolvedModelTarget, userUUID string, chatReq *ChatCompletionRequest, tokenCounter token.ChatTokenCounter, logCapture component.LLMLogRecorder) error {
	if len(modelTarget.AttemptTargets) < 2 {
		return nil
	}
	fallbackTargets := modelTarget.AttemptTargets[1:]
	for idx, fallbackTarget := range fallbackTargets {
		applyChatFallbackTarget(c.Request.Context(), c.Request.Header, modelTarget, fallbackTarget, tokenCounter, logCapture)
		slog.DebugContext(c.Request.Context(), "retrying chat request with fallback endpoint",
			slog.String("model_id", modelTarget.Model.ID),
			slog.String("session_key_hash", modelTarget.SessionKeyHash),
			slog.String("retry_endpoint", modelTarget.Model.Endpoint),
			slog.String("retry_model_name", modelTarget.ModelName))
		retryWriter, err := h.executeChatProxyAttempt(c, w, modelTarget, userUUID, chatReq)
		if err != nil {
			return err
		}
		statusCode := retryWriter.StatusCode()
		streamStarted := retryWriter.StreamStarted()
		retryable := shouldRetryChatAttempt(statusCode, streamStarted)
		if shouldReportChatAttemptFailure(statusCode) {
			h.reportChatAttemptFailure(c.Request.Context(), ChatAttemptFailureEvent{
				Phase:           chatAttemptPhaseFallback,
				ModelID:         modelTarget.Model.ID,
				ModelName:       modelTarget.ModelName,
				Provider:        modelTarget.Model.Provider,
				Endpoint:        modelTarget.Endpoint.URL,
				Target:          modelTarget.Target,
				SessionKeyHash:  modelTarget.SessionKeyHash,
				StatusCode:      statusCode,
				Retryable:       retryable,
				FallbackAttempt: idx + 1,
			})
		}
		isLastFallback := idx == len(fallbackTargets)-1
		// Stop when this attempt has produced a final result for the caller:
		// - on the last fallback, replay any buffered 502/503/504 response because there is no next target;
		// - on success or any non-retryable result, ReplayBufferedResponse becomes a no-op if the response
		//   was already streamed/committed to downstream.
		if isLastFallback || !retryable {
			return retryWriter.ReplayBufferedResponse()
		}
	}
	return nil
}

// GenerateImage godoc
// @Security     ApiKey
// @Summary      Generate image from text prompt
// @Description  Generates images based on a text prompt
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request body  ImageGenerationRequest true "Image generation request"
// @Success      200  {object}  types.ImageGenerationResponse "OK"
// @Failure      400  {object}  error "Bad request or sensitive input"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/images/generations [post]
func (h *OpenAIHandlerImpl) GenerateImage(c *gin.Context) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)
	ctx := c.Request.Context()

	var req ImageGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: err.Error(), Type: "invalid_request_error",
		}})
		return
	}
	if req.Prompt == "" || req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model and prompt cannot be empty", Type: "invalid_request_error",
		}})
		return
	}

	modelID := req.Model
	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		handleModelTargetError(c, ctx, modelID, "failed to get model target address", err)
		return
	}

	adapter := h.t2iRegistry.GetAdapter(modelTarget.Model)
	if adapter == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "unsupported_model", Message: fmt.Sprintf("no adapter for model '%s'", modelID), Type: "invalid_request_error",
		}})
		return
	}

	if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
		h.handleInsufficientBalance(c, false, nsUUID, modelID, err)
		return
	}

	typesReq := types.ImageGenerationRequest{
		ImageGenerateParams: req.ImageGenerateParams,
		RawJSON:             req.RawJSON,
	}
	typesReq.Model = modelTarget.ModelName
	bodyBytes, err := adapter.TransformRequest(ctx, typesReq)
	if err != nil {
		slog.ErrorContext(ctx, "failed to transform image request", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: err.Error(), Type: "internal_error",
		}})
		return
	}

	result, err := h.modComponent.CheckImagePrompts(ctx, req.Prompt, nsUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "moderation_error", Message: "failed to check image prompts: " + err.Error(), Type: "internal_error",
		}})
		return
	}
	if result != nil && result.IsSensitive {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "content_policy_violation", Message: "Input data may contain inappropriate content.", Type: "invalid_request_error",
		}})
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.Request.ContentLength = int64(len(bodyBytes))
	for k, v := range adapter.GetHeaders(modelTarget.Model, &typesReq) {
		c.Request.Header.Set(k, v)
	}

	rp, _ := proxy.NewReverseProxy(modelTarget.Target)
	slog.InfoContext(ctx, "proxy image generation request to model target", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host),
		slog.Any("user", username), slog.Any("model_name", modelTarget.ModelName))

	imageCounter := token.NewImageUsageCounter()
	responseFormat := string(req.ResponseFormat)
	if responseFormat == "" {
		responseFormat = "url"
	}
	imageWrapper := wrapper.NewImageGeneration(c.Writer, adapter, h.modComponent, h.config.AIGateway.SensitiveDefaultImg, imageCounter, responseFormat, h.storage, h.config.S3.Bucket)
	var w http.ResponseWriter = imageWrapper

	proxyToApi := ""
	if modelTarget.Model.Endpoint != "" {
		uri, err := url.ParseRequestURI(modelTarget.Model.Endpoint)
		if err != nil {
			slog.WarnContext(ctx, "endpoint has wrong struct", slog.String("model", modelTarget.ModelName))
		} else {
			proxyToApi = uri.Path
			if proxyToApi == "" {
				// Spaces (HF Inference Toolkit) serve at root.
				proxyToApi = "/"
			}
		}
	}

	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}

	rp.ServeHTTP(w, c.Request, proxyToApi, modelTarget.Host)

	if err := imageWrapper.Finalize(); err != nil {
		slog.ErrorContext(ctx, "failed to finalize image response", slog.Any("error", err))
	}
	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()
		if err := h.openaiComponent.RecordUsage(usageCtx, nsUUID, modelTarget.Model, imageCounter, apikey); err != nil {
			slog.ErrorContext(usageCtx, "failed to record image usage", slog.Any("error", err))
		}
	}()
}

// Embedding godoc
// @Security     ApiKey
// @Summary      Get embedding for a text
// @Description  Sends a text to the backend model and returns the embedding
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request body  EmbeddingRequest true "Embedding request"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  error "Bad request or sensitive input"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/embeddings [post]
func (h *OpenAIHandlerImpl) Embedding(c *gin.Context) {
	var req EmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model cannot be empty"})
		return
	}
	if req.Input.OfString.String() == "" &&
		len(req.Input.OfArrayOfStrings) == 0 &&
		len(req.Input.OfArrayOfTokenArrays) == 0 &&
		len(req.Input.OfArrayOfTokens) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input cannot be empty"})
		return
	}
	modelID := req.Model
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)
	modelTarget, err := h.resolveModelTarget(c.Request.Context(), username, modelID, c.Request.Header)
	if err != nil {
		handleModelTargetError(c, c.Request.Context(), modelID, "failed to get embedding target address", err)
		return
	}

	// Check balance before processing request
	if err := h.openaiComponent.CheckBalance(c.Request.Context(), nsUUID); err != nil {
		h.handleInsufficientBalance(c, false, nsUUID, modelID, err)
		return
	}

	req.Model = modelTarget.ModelName
	data, _ := json.Marshal(req)
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Request.ContentLength = int64(len(data))
	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(c.Request.Context(), "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}
	slog.InfoContext(c, "proxy embedding request to model endpoint", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host), slog.Any("user", username), slog.Any("model_id", modelID))
	rp, _ := proxy.NewReverseProxy(modelTarget.Target)

	tokenCounter := h.tokenCounterFactory.NewEmbedding(token.CreateParam{
		Endpoint: modelTarget.Target,
		Host:     modelTarget.Host,
		Model:    modelTarget.ModelName,
		ImageID:  modelTarget.Model.ImageID,
		Provider: modelTarget.Model.Provider,
	})
	w := NewResponseWriterWrapperEmbedding(c.Writer, tokenCounter)
	if req.Input.OfString.String() != "" {
		tokenCounter.Input(req.Input.OfString.Value)
	}

	rp.ServeHTTP(w, c.Request, "", modelTarget.Host)
	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		err := h.openaiComponent.RecordUsage(usageCtx, nsUUID, modelTarget.Model, tokenCounter, apikey)
		if err != nil {
			slog.ErrorContext(c, "failed to record embedding token usage", "error", err)
		}
	}()
}

// Transcription godoc
// @Security     ApiKey
// @Summary      Transcribe audio to text
// @Description  Sends an OpenAI-compatible multipart audio transcription request to the backend model
// @Tags         AIGateway
// @Accept       multipart/form-data
// @Produce      json
// @Param        model formData string true "Model ID"
// @Param        file formData file true "Audio file"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  error "Bad request"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/audio/transcriptions [post]
func (h *OpenAIHandlerImpl) Transcription(c *gin.Context) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)
	ctx := c.Request.Context()

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: "invalid multipart form: " + err.Error(),
			Type:    "invalid_request_error",
		}})
		return
	}
	if form == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: "request must be multipart/form-data",
			Type:    "invalid_request_error",
		}})
		return
	}

	modelID := strings.TrimSpace(firstMultipartValue(form, "model"))
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: "Model cannot be empty",
			Type:    "invalid_request_error",
		}})
		return
	}
	if len(form.File["file"]) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: "File cannot be empty",
			Type:    "invalid_request_error",
		}})
		return
	}

	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		handleModelTargetError(c, ctx, modelID, "failed to get transcription target address", err)
		return
	}

	if !modelTarget.Model.SkipBalance() {
		if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
			h.handleInsufficientBalance(c, false, nsUUID, modelID, err)
			return
		}
	}

	body, contentType := rewriteMultipartModelStream(form, modelTarget.ModelName)
	c.Request.Body = body
	c.Request.ContentLength = -1
	c.Request.Header.Set("Content-Type", contentType)
	c.Request.Header.Del("Content-Length")

	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}

	rp, err := proxy.NewReverseProxy(modelTarget.Target, proxy.WithoutAcceptEncoding())
	if err != nil {
		slog.ErrorContext(ctx, "failed to create reverse proxy", slog.Any("error", err))
		c.String(http.StatusInternalServerError, fmt.Errorf("failed to create reverse proxy:%w", err).Error())
		return
	}

	proxyToApi := ""
	if modelTarget.Model.Endpoint != "" {
		uri, err := url.ParseRequestURI(modelTarget.Model.Endpoint)
		if err != nil {
			slog.WarnContext(ctx, "endpoint has wrong struct", slog.String("model", modelTarget.ModelName))
		} else {
			proxyToApi = uri.Path
		}
	}

	slog.InfoContext(ctx, "proxy audio transcription request to model endpoint", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host), slog.Any("user", username), slog.Any("model_id", modelID), slog.Any("model_name", modelTarget.ModelName))

	audioCounter := token.NewAudioUsageCounter(token.NewTokenizerImpl(modelTarget.Target, modelTarget.Host, modelTarget.ModelName, modelTarget.Model.ImageID, modelTarget.Model.Provider))
	w := NewResponseWriterWrapperAudio(c.Writer, audioCounter)
	rp.ServeHTTP(w, c.Request, proxyToApi, modelTarget.Host)

	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		if err := h.openaiComponent.RecordUsage(usageCtx, nsUUID, modelTarget.Model, audioCounter, apikey); err != nil {
			slog.ErrorContext(usageCtx, "failed to record audio transcription usage", slog.Any("error", err))
		}
	}()
}
