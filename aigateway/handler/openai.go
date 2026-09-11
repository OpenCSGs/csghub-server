package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"opencsg.com/csghub-server/aigateway/component/router"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/aigateway/component"
	audioadapter "opencsg.com/csghub-server/aigateway/component/adapter/audio"
	ocradapter "opencsg.com/csghub-server/aigateway/component/adapter/ocr"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2image"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2video"
	"opencsg.com/csghub-server/aigateway/component/availability"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	commonType "opencsg.com/csghub-server/common/types"
	apicomp "opencsg.com/csghub-server/component"
)

// OpenAIHandler defines the interface for handling OpenAI compatible APIs
type OpenAIHandler interface {
	// List available models
	ListModels(c *gin.Context)
	// Get model details
	GetModel(c *gin.Context)
	// Get embedding for a text
	Embedding(c *gin.Context)
	// Rerank documents against a query for a text-ranking model
	Rerank(c *gin.Context)
	// Get a video generation
	GetVideo(c *gin.Context)
	// Download generated video content
	GetVideoContent(c *gin.Context)
	// List available voices of a text-to-speech model
	ListVoices(c *gin.Context)
	// Upload a voice sample for voice cloning
	UploadVoice(c *gin.Context)
	// Update an uploaded voice sample by name
	UpdateVoice(c *gin.Context)
	// Delete an uploaded voice sample by name
	DeleteVoice(c *gin.Context)
	// Set chat attempt failure reporter
	SetChatAttemptFailureReporter(reporter ChatAttemptFailureReporter)
	// Shutdown releases handler-owned resources.
	Shutdown(ctx context.Context) error
}

func NewOpenAIHandlerFromConfig(config *config.Config) (*OpenAIHandlerImpl, error) {
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
	aiGenerationStore := database.NewAIGenerationStore()
	handler := newOpenAIHandler(modelService, repoComp, modComponent, clusterComp, token.NewCounterFactory(), text2image.NewRegistry(), text2video.NewRegistry(), audioadapter.NewRegistry(), config, storage, whitelistRule, aiGenerationStore)

	if config.AIGateway.EnableLLMTrace && config.Instrumentation.OTLPEndpoint != "" {
		llmTracer, traceErr := llmtrace.NewSigilTracer(llmtrace.SigilConfig{
			ContentCapture:       config.AIGateway.LLMTraceContentCapture,
			MaxContentLength:     config.AIGateway.LLMTraceMaxContentLength,
			MaxInputUserMessages: config.AIGateway.LLMTraceMaxInputUserMessages,
			ToolDefinitionsMode:  config.AIGateway.LLMTraceToolDefinitionsMode,
		})
		if traceErr != nil {
			slog.Warn("failed to create llm tracer", slog.Any("error", traceErr))
		} else {
			handler.llmTracer = llmTracer
		}
	}

	availabilityManager, avErr := availability.NewAvailabilityManagerFromConfig(config)
	if avErr != nil {
		slog.Warn("failed to initialize availability manager", "error", avErr)
		return handler, nil
	}
	if startErr := availabilityManager.Start(context.Background()); startErr != nil {
		slog.Warn("failed to start availability manager", "error", startErr)
	} else {
		handler.availabilityManager = availabilityManager
	}
	return handler, nil
}

func newOpenAIHandler(
	modelService component.OpenAIComponent,
	repoComp apicomp.RepoComponent,
	modComponent component.Moderation,
	clusterComp apicomp.ClusterComponent,
	tokenCounterFactory token.CounterFactory,
	t2iRegistry *text2image.Registry,
	t2vRegistry *text2video.Registry,
	audioRegistry *audioadapter.Registry,
	config *config.Config,
	storage types.Storage,
	whitelistRule database.RepositoryFileCheckRuleStore,
	aiGenerationStore database.AIGenerationStore,
) *OpenAIHandlerImpl {
	return &OpenAIHandlerImpl{
		openaiComponent:            modelService,
		repoComp:                   repoComp,
		modComponent:               modComponent,
		clusterComp:                clusterComp,
		tokenCounterFactory:        tokenCounterFactory,
		t2iRegistry:                t2iRegistry,
		t2vRegistry:                t2vRegistry,
		audioRegistry:              audioRegistry,
		config:                     config,
		storage:                    storage,
		whitelistRule:              whitelistRule,
		aiGenerationStore:          aiGenerationStore,
		sensitivePolicy:            component.NewSensitivePolicy(modComponent, whitelistRule),
		ocrRegistry:                ocradapter.NewRegistry(),
		llmLogPublisher:            component.NewLLMLogPublisher(),
		sessionRouter:              router.NewSessionRouter(),
		chatAttemptFailureReporter: noopChatAttemptFailureReporter{},
	}
}

func (h *OpenAIHandlerImpl) Shutdown(ctx context.Context) error {
	if h == nil || h.llmTracer == nil {
		return nil
	}
	return h.llmTracer.Shutdown(ctx)
}

func insufficientBalanceMessage(frontendURL string) string {
	rechargeURL := strings.TrimRight(frontendURL, "/") + "/settings/recharge-payment"
	return fmt.Sprintf(
		"**Insufficient balance**\n\n👉 [Recharge your account](%s) to continue.",
		rechargeURL,
	)
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
	t2vRegistry                *text2video.Registry
	audioRegistry              *audioadapter.Registry
	ocrRegistry                *ocradapter.Registry
	config                     *config.Config
	storage                    types.Storage
	whitelistRule              database.RepositoryFileCheckRuleStore
	aiGenerationStore          database.AIGenerationStore
	sensitivePolicy            component.SensitivePolicy
	llmLogPublisher            component.LLMLogPublisher
	sessionRouter              router.SessionRouter
	availabilityManager        availability.AvailabilityManager
	chatAttemptFailureReporter ChatAttemptFailureReporter
	llmTracer                  llmtrace.LLMTracer
	responsesIDMapper          *responsespkg.IDMapper
	responsesIDMapperOnce      sync.Once
	responsesIDMapperErr       error
}

// ListModels godoc
// @Summary      List available models
// @Description  Returns a list of available models, supports fuzzy search by model_id query parameter and filtering by llm_types, task, and associated CSGHub model repository
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        model_id query string false "Model ID for fuzzy search"
// @Param        llm_types query []string false "Filter by LLM types" Enums(external_llm, serverless, inference)
// @Param        task query string false "Filter by task (e.g., text-generation, text-to-image, image-to-image)"
// @Param        has_associated_model query bool false "Filter by whether models are linked to a CSGHub model repository"
// @Param        per query int false "Models per page, must be provided with page (max 100)"
// @Param        page query int false "Page number, must be provided with per (1-based)"
// @Success      200  {object}  types.ModelList "OK"
// @Failure      400  {object}  error "Invalid llm_types, has_associated_model, or pagination parameter"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/models [get]
func (h *OpenAIHandlerImpl) ListModels(c *gin.Context) {
	currentUser := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	if len(nsUUID) < 1 {
		nsUUID = httpbase.GetCurrentUserUUID(c)
	}
	// Validate llm_types parameter
	llmTypes := c.QueryArray("llm_types")
	for _, llmType := range llmTypes {
		if strings.TrimSpace(llmType) == "" {
			continue
		}
		if !isValidListModelsLLMType(llmType) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": types.Error{
					Code:    "invalid_request_error",
					Message: invalidLLMTypesErrorMessage(),
					Type:    "invalid_request_error",
				}})
			return
		}
	}

	var hasAssociatedModel *bool
	if hasAssociatedModelQuery := c.Query("has_associated_model"); hasAssociatedModelQuery != "" {
		parsed, parseErr := strconv.ParseBool(hasAssociatedModelQuery)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": types.Error{
					Code:    "invalid_request_error",
					Message: invalidHasAssociatedModelErrorMessage(),
					Type:    "invalid_request_error",
				}})
			return
		}
		hasAssociatedModel = &parsed
	}

	per, page, paginationErr := parseListModelsPagination(c)
	if paginationErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": *paginationErr,
		})
		return
	}

	resp, err := h.openaiComponent.ListModels(c.Request.Context(), nsUUID, types.ListModelsReq{
		ModelID:            c.Query("model_id"),
		LLMTypes:           llmTypes,
		Task:               c.Query("task"),
		HasAssociatedModel: hasAssociatedModel,
		Per:                per,
		Page:               page,
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

func isValidListModelsLLMType(llmType string) bool {
	switch strings.ToLower(strings.TrimSpace(llmType)) {
	case commonType.ProviderTypeExternalLLM, commonType.ProviderTypeServerless, commonType.ProviderTypeInference:
		return true
	default:
		return false
	}
}

func invalidLLMTypesErrorMessage() string {
	return fmt.Sprintf("Invalid llm_types parameter. Allowed values: %s, %s, %s", commonType.ProviderTypeExternalLLM, commonType.ProviderTypeServerless, commonType.ProviderTypeInference)
}

func invalidHasAssociatedModelErrorMessage() string {
	return "Invalid has_associated_model parameter. Allowed values: true, false"
}

func parseListModelsPagination(c *gin.Context) (int, int, *types.Error) {
	perValue, hasPer := c.GetQuery("per")
	pageValue, hasPage := c.GetQuery("page")
	if hasPer != hasPage {
		return 0, 0, &types.Error{
			Code:    "invalid_request_error",
			Message: "per and page must be provided together",
			Type:    "invalid_request_error",
		}
	}
	if !hasPer {
		return 0, 0, nil
	}

	per, perErr := strconv.Atoi(perValue)
	page, pageErr := strconv.Atoi(pageValue)
	if perErr != nil || pageErr != nil || per <= 0 || page <= 0 {
		return 0, 0, &types.Error{
			Code:    "invalid_request_error",
			Message: "Invalid pagination parameter. per and page must be positive integers",
			Type:    "invalid_request_error",
		}
	}
	return per, page, nil
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
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	if len(nsUUID) < 1 {
		nsUUID = httpbase.GetCurrentUserUUID(c)
	}
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

	model, err := h.openaiComponent.GetModelByID(c.Request.Context(), nsUUID, modelID)
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

	c.PureJSON(http.StatusOK, model)
}

var _ openai.ChatCompletion
var _ openai.ChatCompletionChunk

type chatPostProcessInput struct {
	NSUUID          string
	ApiKey          string
	Model           *types.Model
	TargetModelName string
	TokenCounter    token.Counter
	Usage           *token.Usage
	LogCapture      component.LLMLogRecorder
	Trace           chatTracePostProcessInput
	StatusCode      int
}

type chatContext struct {
	tokenCounter   token.ChatTokenCounter
	logCapture     component.LLMLogRecorder
	responseWriter CommonResponseWriter
}

func (h *OpenAIHandlerImpl) setupChatContext(
	ctx context.Context,
	modelTarget *resolvedModelTarget,
	chatReq *types.ChatCompletionRequest,
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
	chatReq *types.ChatCompletionRequest,
	primaryWriter *chatRetryResponseWriter,
	username string,
	modelID string,
) (*chatRetryResponseWriter, error) {
	primaryStatusCode := primaryWriter.StatusCode()
	primaryStreamStarted := primaryWriter.StreamStarted()
	primaryRetryable := shouldRetryChatAttempt(primaryStatusCode, primaryStreamStarted)

	h.reportChatAttemptResult(c.Request.Context(), chatAttemptReportParams{
		UpstreamID:     modelTarget.Upstream.ID,
		Phase:          chatAttemptPhasePrimary,
		RequestModelID: modelID,
		ModelName:      modelTarget.ModelName,
		Provider:       modelTarget.Model.Provider,
		Endpoint:       modelTarget.Upstream.URL,
		Target:         modelTarget.Target,
		StatusCode:     primaryStatusCode,
		Retryable:      primaryRetryable,
		Model:          modelTarget.Model,
	})

	hasFallbacks := len(modelTarget.AttemptTargets) > 0
	if !primaryRetryable || !hasFallbacks {
		if replayErr := primaryWriter.ReplayBufferedResponse(); replayErr != nil {
			slog.WarnContext(c.Request.Context(), "failed to replay buffered response", slog.Any("error", replayErr))
		}
		return primaryWriter, nil
	}

	slog.InfoContext(c.Request.Context(), "retry chat request with fallback endpoint",
		slog.String("model_id", modelID),
		slog.String("user_name", username),
		slog.Int("available_fallback_attempts", len(modelTarget.AttemptTargets)),
		slog.String("retry_reason", chatRetryReason(primaryStatusCode)),
		slog.Int("status_code", primaryStatusCode))

	retryWriter, retryErr := h.retryChatWithFallback(c, chatCtx.responseWriter, modelTarget, userUUID, chatReq, chatCtx.tokenCounter, chatCtx.logCapture)
	if retryErr != nil {
		if component.IsUsageLimitExceeded(retryErr) {
			return nil, retryErr
		}
		slog.ErrorContext(c.Request.Context(), "fallback chat retry failed", slog.Any("error", retryErr))
		if replayErr := primaryWriter.ReplayBufferedResponse(); replayErr != nil {
			slog.WarnContext(c.Request.Context(), "failed to replay buffered fallback response after retry error", slog.Any("error", replayErr))
		}
		return primaryWriter, nil
	}
	return retryWriter, nil
}

// preComputeUsage synchronously calls counter.Usage() with a short timeout
// so the resulting *Usage pointer can be shared between the sync metrics path
// and the async post-process goroutine, avoiding duplicate computation.
// Returns nil if the counter is nil or the call fails/times out.
func preComputeUsage(ctx context.Context, counter token.Counter) *token.Usage {
	if counter == nil {
		return nil
	}
	usageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	usage, err := counter.Usage(usageCtx)
	if err != nil {
		slog.DebugContext(usageCtx, "pre-compute token usage failed", slog.Any("error", err))
		return nil
	}
	return usage
}

func (h *OpenAIHandlerImpl) runChatPostProcessAsync(ctx context.Context, input chatPostProcessInput) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic in chat post-process", slog.Any("panic", r))
				if input.Trace.Recorder != nil {
					input.Trace.Recorder.End()
				}
			}
		}()

		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()

		// Use the pre-computed usage from the sync path when available;
		// fall back to a fresh counter.Usage() call only when the sync
		// pre-compute failed (nil).
		usage := input.Usage
		if usage == nil && input.TokenCounter != nil {
			var usageErr error
			usage, usageErr = input.TokenCounter.Usage(usageCtx)
			if usageErr != nil {
				slog.ErrorContext(usageCtx, "failed to get chat token usage", slog.Any("error", usageErr))
			}
		}
		if input.Trace.Recorder != nil {
			provider := ""
			if input.Model != nil {
				provider = input.Model.Provider
			}
			var inputMsgs, outputMsgs []types.GenerationMessage
			var traceInfo commonType.LLMLogTraceInfo
			if input.LogCapture != nil {
				in, out := input.LogCapture.Messages()
				inputMsgs = llmlogMessagesToGenerationMessages(in)
				outputMsgs = llmlogMessagesToGenerationMessages(out)
				traceInfo = input.LogCapture.TraceInfo()
			}
			recordChatTraceCompletion(input.Trace, provider, input.TargetModelName, usage, inputMsgs, outputMsgs, traceInfo)
			input.Trace.Recorder.End()
		}

		if err := h.openaiComponent.CommitUsageLimitFromUsage(usageCtx, input.NSUUID, input.Model, usage); err != nil {
			slog.ErrorContext(usageCtx, "failed to commit usage limit", slog.Any("error", err))
		}

		if usage != nil && isSuccessfulStatus(input.StatusCode) {
			if err := h.openaiComponent.RecordUsageFromTokenUsage(usageCtx, input.NSUUID, input.Model, input.TargetModelName, usage, input.ApiKey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record token usage", slog.Any("error", err))
			}
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

func (h *OpenAIHandlerImpl) executeChatProxyAttempt(c *gin.Context, w CommonResponseWriter, modelTarget *resolvedModelTarget, userUUID string, chatReq *types.ChatCompletionRequest) (*chatRetryResponseWriter, error) {
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

func (h *OpenAIHandlerImpl) retryChatWithFallback(c *gin.Context, w CommonResponseWriter, modelTarget *resolvedModelTarget, userUUID string, chatReq *types.ChatCompletionRequest, tokenCounter token.ChatTokenCounter, logCapture component.LLMLogRecorder) (*chatRetryResponseWriter, error) {
	if len(modelTarget.AttemptTargets) < 1 {
		return nil, nil
	}
	fallbackTargets := modelTarget.AttemptTargets
	for idx, fallbackTarget := range fallbackTargets {
		applyChatFallbackTarget(c.Request.Context(), c.Request.Header, modelTarget, fallbackTarget, tokenCounter, logCapture)
		slog.DebugContext(c.Request.Context(), "retrying chat request with fallback endpoint",
			slog.String("model_id", modelTarget.Model.ID),
			slog.String("retry_endpoint", modelTarget.Model.Endpoint),
			slog.String("retry_model_name", modelTarget.ModelName))
		retryWriter, err := h.executeChatProxyAttempt(c, w, modelTarget, userUUID, chatReq)
		if err != nil {
			return nil, err
		}
		statusCode := retryWriter.StatusCode()
		streamStarted := retryWriter.StreamStarted()
		retryable := shouldRetryChatAttempt(statusCode, streamStarted)
		h.reportChatAttemptResult(c.Request.Context(), chatAttemptReportParams{
			UpstreamID:      modelTarget.Upstream.ID,
			Phase:           chatAttemptPhaseFallback,
			ModelName:       modelTarget.ModelName,
			Provider:        modelTarget.Model.Provider,
			Endpoint:        modelTarget.Upstream.URL,
			Target:          modelTarget.Target,
			StatusCode:      statusCode,
			Retryable:       retryable,
			FallbackAttempt: idx + 1,
			Model:           modelTarget.Model,
		})
		isLastFallback := idx == len(fallbackTargets)-1
		// Stop when this attempt has produced a final result for the caller:
		// - on the last fallback, replay any buffered 502/503/504 response because there is no next target;
		// - on success or any non-retryable result, ReplayBufferedResponse becomes a no-op if the response
		//   was already streamed/committed to downstream.
		if isLastFallback || !retryable {
			return retryWriter, retryWriter.ReplayBufferedResponse()
		}
	}
	return nil, nil
}

func resolveFailureEventModelID(requestModelID string, model *types.Model) string {
	if trimmed := strings.TrimSpace(requestModelID); trimmed != "" {
		return trimmed
	}
	if model == nil {
		return ""
	}
	return strings.TrimSpace(model.ID)
}

// retryWriterStatusCode safely extracts the status code from a chatRetryResponseWriter.
// Returns 0 if the writer is nil (e.g., when an error occurs before any response is written).
func retryWriterStatusCode(w *chatRetryResponseWriter) int {
	if w != nil {
		return w.statusCode
	}
	return 0
}

// retryWriterTTFTMs safely computes TTFT (Time To First Token) in milliseconds.
// Returns 0 if writer is nil or firstWriteAt is zero.
func retryWriterTTFTMs(w *chatRetryResponseWriter, startTime time.Time) int64 {
	if w == nil {
		return 0
	}
	firstWriteAt := w.firstWriteAt
	if firstWriteAt.IsZero() {
		return 0
	}
	return firstWriteAt.Sub(startTime).Milliseconds()
}
