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
	// Responses runs OpenAI-compatible Responses API requests.
	Responses(c *gin.Context)
	// Get embedding for a text
	Embedding(c *gin.Context)
	// Rerank documents against a query for a text-ranking model
	Rerank(c *gin.Context)
	// Generate image from text
	GenerateImage(c *gin.Context)
	// Edit image from prompt and input image
	EditImage(c *gin.Context)
	// Create a video generation
	CreateVideo(c *gin.Context)
	// Get a video generation
	GetVideo(c *gin.Context)
	// Download generated video content
	GetVideoContent(c *gin.Context)
	// Transcribe audio to text
	Transcription(c *gin.Context)
	// Extract text from an image with OCR
	OCR(c *gin.Context)
	// Generate speech audio from text
	Speech(c *gin.Context)
	// Generate speech audio for multiple texts in a single request
	SpeechBatch(c *gin.Context)
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

	resp, err := h.openaiComponent.ListModels(c.Request.Context(), currentUser, types.ListModelsReq{
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
	ctx := c.Request.Context()
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)
	requestID := trace.GetTraceIDInGinContext(c)
	ctx, preflight := startPreflightTrace(ctx, preflightTraceStart{
		API:       c.FullPath(),
		RequestID: requestID,
		UserID:    nsUUID,
	})
	c.Request = c.Request.WithContext(ctx)

	chatReq := &ChatCompletionRequest{}
	if err := c.BindJSON(chatReq); err != nil {
		slog.ErrorContext(ctx, "invalid chat completion request body", slog.Any("error", err))
		preflight.RecordError(err, "bad_request")
		c.String(http.StatusBadRequest, fmt.Errorf("invalid chat completion request body:%w", err).Error())
		return
	}
	modelID := chatReq.Model

	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		preflight.RecordError(err, "model_resolve")
		handleModelTargetError(c, ctx, modelID, "failed to get model target address", err)
		return
	}
	applyChatCompletionsEndpointCompatibility(ctx, modelTarget)
	preflight.SetTargetModel(modelID, modelTarget)
	chatReq.Model = modelTarget.ModelName
	if chatReq.Stream {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		if !strings.Contains(modelTarget.Model.ImageID, "vllm-cpu") {
			chatReq.StreamOptions = &StreamOptions{
				IncludeUsage: true,
			}
		}
	}

	preflight.End()

	traceCtx, generationRecorder := h.startChatTrace(
		ctx,
		c.Request.Header,
		modelID,
		modelTarget,
		chatReq,
		requestID,
		nsUUID,
	)
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	// Check balance before processing request
	if !modelTarget.Model.SkipBalance() {
		if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
			finishLLMTraceWithError(generationRecorder, err, types.TraceErrInsufficientBalance)
			h.handleInsufficientBalance(c, chatReq.Stream, nsUUID, modelID, err)
			return
		}
	}
	var modComponent component.Moderation = nil
	isCheck, result, err := h.sensitivePolicy.CheckChatSensitive(ctx, modelTarget.Model, chatReq.Messages, nsUUID, chatReq.Stream, modelTarget.Upstream.Provider)
	if err != nil {
		slog.ErrorContext(ctx, "failed to check sensitive",
			slog.String("model_id", modelID),
			slog.String("username", username),
			slog.Any("error", err))
	}
	if isCheck {
		modComponent = h.modComponent
		if result != nil && result.IsSensitive {
			finishLLMTraceWithError(generationRecorder, ErrSensitiveContent, types.TraceErrSensitivePrompt)
			handleSensitiveResponse(c, chatReq.Stream, result)
			return
		}
	}

	chatCtx := h.setupChatContext(
		ctx,
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
		slog.WarnContext(ctx, "invalid auth head",
			slog.String("model", modelTarget.ModelName),
			slog.Any("error", err))
	}

	proxyStartTime := time.Now()
	log := slog.With(
		slog.String("proxy_start_time", proxyStartTime.Format(time.RFC3339)),
		slog.Any("model_name", modelTarget.ModelName),
		slog.Any("current_user", username),
		slog.Any("target", modelTarget.Target),
		slog.Any("host", modelTarget.Host),
	)
	primaryWriter, proxyErr := h.executeChatProxyAttempt(c, chatCtx.responseWriter, modelTarget, nsUUID, chatReq)
	if proxyErr != nil {
		finishLLMTraceWithError(generationRecorder, proxyErr, types.TraceErrUpstreamUnavailable)
		h.handleProxyError(c, chatReq.Stream, username, modelID, proxyErr)
		log.ErrorContext(ctx, "failed to execute chat proxy", slog.Int("status", retryWriterStatusCode(primaryWriter)), slog.Any("error", proxyErr))
		return
	}
	log.InfoContext(ctx, "proxy chat request to model target", slog.Int("status", primaryWriter.statusCode), slog.Int64("proxy_latency(ms)", time.Since(proxyStartTime).Milliseconds()), slog.Int64("ttft(ms)", retryWriterTTFTMs(primaryWriter, proxyStartTime)))

	finalWriter, err := h.executeChatWithFallback(c, chatCtx, modelTarget, nsUUID, chatReq, primaryWriter, username, modelID)
	if err != nil {
		finishLLMTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		h.handleProxyError(c, chatReq.Stream, username, modelID, err)
		log.ErrorContext(ctx, "failed to execute chat fallback", slog.Int("status", retryWriterStatusCode(finalWriter)), slog.Any("error", err))
		return
	}
	log.InfoContext(ctx, "fallback chat request to model target", slog.Int("status", retryWriterStatusCode(finalWriter)), slog.Int64("proxy_latency(ms)", time.Since(proxyStartTime).Milliseconds()), slog.Int64("ttft(ms)", retryWriterTTFTMs(finalWriter, proxyStartTime)))

	h.runChatPostProcessAsync(ctx, chatPostProcessInput{
		NSUUID:          nsUUID,
		ApiKey:          apikey,
		Model:           modelTarget.Model,
		TargetModelName: modelTarget.ModelName,
		TokenCounter:    chatCtx.tokenCounter,
		LogCapture:      chatCtx.logCapture,
		Trace:           newChatTracePostProcessInput(generationRecorder, chatReq, finalWriter),
	})
}

type chatPostProcessInput struct {
	NSUUID          string
	ApiKey          string
	Model           *types.Model
	TargetModelName string
	TokenCounter    token.Counter
	LogCapture      component.LLMLogRecorder
	Trace           chatTracePostProcessInput
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

		var usage *token.Usage
		if input.TokenCounter != nil {
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

		if err := h.openaiComponent.CommitUsageLimit(usageCtx, input.NSUUID, input.Model, input.TokenCounter); err != nil {
			slog.ErrorContext(usageCtx, "failed to commit usage limit", slog.Any("error", err))
		}

		if usage != nil {
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

func (h *OpenAIHandlerImpl) retryChatWithFallback(c *gin.Context, w CommonResponseWriter, modelTarget *resolvedModelTarget, userUUID string, chatReq *ChatCompletionRequest, tokenCounter token.ChatTokenCounter, logCapture component.LLMLogRecorder) (*chatRetryResponseWriter, error) {
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
	ctx := c.Request.Context()
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)
	requestID := trace.GetTraceIDInGinContext(c)
	ctx, preflight := startPreflightTrace(ctx, preflightTraceStart{
		API:       c.FullPath(),
		RequestID: requestID,
		UserID:    nsUUID,
	})
	c.Request = c.Request.WithContext(ctx)

	var req EmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		preflight.RecordError(err, "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Model == "" {
		preflight.RecordError(fmt.Errorf("model cannot be empty"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model cannot be empty"})
		return
	}
	if req.Input.OfString.String() == "" &&
		len(req.Input.OfArrayOfStrings) == 0 &&
		len(req.Input.OfArrayOfTokenArrays) == 0 &&
		len(req.Input.OfArrayOfTokens) == 0 {
		preflight.RecordError(fmt.Errorf("input cannot be empty"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input cannot be empty"})
		return
	}
	modelID := req.Model
	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		preflight.RecordError(err, "model_resolve")
		handleModelTargetError(c, ctx, modelID, "failed to get embedding target address", err)
		return
	}

	preflight.SetTargetModel(modelID, modelTarget)
	preflight.End()

	traceCtx, embeddingRecorder := h.startEmbeddingTrace(
		ctx,
		modelID,
		modelTarget,
		&req,
		requestID,
		nsUUID,
	)
	c.Request = c.Request.WithContext(traceCtx)

	// Check balance before processing request
	if err := h.openaiComponent.CheckBalance(c.Request.Context(), nsUUID); err != nil {
		finishEmbeddingTraceWithError(embeddingRecorder, err, types.TraceErrInsufficientBalance)
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
	proxyToAPI := resolveProxyPathFromModelEndpoint(modelTarget.Model.Endpoint, modelTarget.ModelName)
	rp, err := proxy.NewReverseProxy(modelTarget.Target, proxy.WithoutAcceptEncoding())
	if err != nil {
		finishEmbeddingTraceWithError(embeddingRecorder, err, types.TraceErrUpstreamUnavailable)
		httpbase.ServerError(c, err)
		return
	}

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

	rp.ServeHTTP(w, c.Request, proxyToAPI, modelTarget.Host)
	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		w.CaptureEmbeddingUsage()

		if embeddingRecorder != nil {
			var usage *token.Usage
			if tokenCounter != nil {
				var usageErr error
				usage, usageErr = tokenCounter.Usage(usageCtx)
				if usageErr != nil {
					slog.ErrorContext(usageCtx, "failed to get embedding token usage", slog.Any("error", usageErr))
				}
			}
			recordEmbeddingTraceCompletion(embeddingRecorder, &req, modelTarget.ModelName, usage, w.StatusCode())
			embeddingRecorder.End()
		}

		err := h.openaiComponent.RecordUsage(usageCtx, nsUUID, modelTarget.Model, modelTarget.ModelName, tokenCounter, apikey)
		if err != nil {
			slog.ErrorContext(c, "failed to record embedding token usage", "error", err)
		}
	}()
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
