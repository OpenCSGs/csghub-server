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
		openaiComponent:     modelService,
		repoComp:            repoComp,
		modComponent:        modComponent,
		clusterComp:         clusterComp,
		tokenCounterFactory: tokenCounterFactory,
		t2iRegistry:         t2iRegistry,
		config:              config,
		storage:             storage,
		whitelistRule:       whitelistRule,
		llmLogPublisher:     component.NewLLMLogPublisher(),
	}
}

// handleInsufficientBalance handles the insufficient balance error response
// for both stream and non-stream requests
func (h *OpenAIHandlerImpl) handleInsufficientBalance(c *gin.Context, isStream bool, username, modelID string, err error) {
	// Check if the error is the standard insufficient balance error
	if !errors.Is(err, errorx.ErrInsufficientBalance) {
		// If it's a different error, log and return generic error
		slog.ErrorContext(c.Request.Context(), "balance check failed",
			"user", username, "model", modelID, "error", err)
		httpbase.ServerError(c, err)
		return
	}

	slog.WarnContext(c.Request.Context(), "insufficient balance for request",
		"user", username, "model", modelID)

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

func (h *OpenAIHandlerImpl) checkSensitive(ctx context.Context, model *types.Model, chatReq *ChatCompletionRequest, userUUID string, stream bool) (bool, *rpc.CheckResult, error) {
	if !model.NeedSensitiveCheck {
		return false, nil, nil
	}

	namespaceTargets := buildNamespaceTargets(model.OfficialName, model.ID)
	rules, err := h.whitelistRule.ListBySensitiveCheckTargets(ctx, namespaceTargets, model.ID)
	if err != nil {
		return false, nil, fmt.Errorf("failed to query white list rules: %w", err)
	}
	if len(rules) != 0 {
		slog.DebugContext(ctx, "Skip Sensitive check with white list", slog.Any("rule", rules[0]))
		return false, nil, nil
	}

	key := fmt.Sprintf("%s:%s", userUUID, model.ID)
	result, err := h.modComponent.CheckChatPrompts(ctx, chatReq.Messages, key, stream)
	if err != nil {
		return false, nil, fmt.Errorf("failed to call moderation error:%w", err)
	}

	return true, result, nil
}

func buildNamespaceTargets(officialName, modelID string) []string {
	targetSet := make(map[string]struct{}, 2)
	targets := make([]string, 0, 2)
	if namespace := extractNamespaceTarget(officialName); namespace != "" {
		if _, exists := targetSet[namespace]; !exists {
			targetSet[namespace] = struct{}{}
			targets = append(targets, namespace)
		}
	}
	if namespace := extractNamespaceTarget(modelID); namespace != "" {
		if _, exists := targetSet[namespace]; !exists {
			targetSet[namespace] = struct{}{}
			targets = append(targets, namespace)
		}
	}
	return targets
}

func extractNamespaceTarget(path string) string {
	normalizedPath := strings.Trim(strings.TrimSpace(path), "/")
	if normalizedPath == "" {
		return ""
	}
	parts := strings.Split(normalizedPath, "/")
	if len(parts) == 0 {
		return ""
	}
	namespace := strings.ToLower(strings.TrimSpace(parts[0]))
	if namespace == "" {
		return ""
	}
	return namespace
}

// OpenAIHandlerImpl implements the OpenAIHandler interface
type OpenAIHandlerImpl struct {
	openaiComponent     component.OpenAIComponent
	repoComp            apicomp.RepoComponent
	modComponent        component.Moderation
	clusterComp         apicomp.ClusterComponent
	tokenCounterFactory token.CounterFactory
	t2iRegistry         *text2image.Registry
	config              *config.Config
	storage             types.Storage
	whitelistRule       database.RepositoryFileCheckRuleStore
	llmLogPublisher     component.LLMLogPublisher
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
	userUUID := httpbase.GetCurrentUserUUID(c)
	chatReq := &ChatCompletionRequest{}
	if err := c.BindJSON(chatReq); err != nil {
		slog.ErrorContext(c.Request.Context(), "invalid chat compoletion request body", "error", err.Error())
		c.String(http.StatusBadRequest, fmt.Errorf("invalid chat compoletion request body:%w", err).Error())
		return
	}
	modelID := chatReq.Model
	modelTarget, err := h.resolveModelTarget(c.Request.Context(), username, modelID)
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
		if err := h.openaiComponent.CheckBalance(c.Request.Context(), username, userUUID); err != nil {
			h.handleInsufficientBalance(c, chatReq.Stream, username, modelID, err)
			return
		}
	}

	// marshal updated request map back to JSON bytes
	updatedBodyBytes, _ := json.Marshal(chatReq)
	c.Request.Body = io.NopCloser(bytes.NewReader(updatedBodyBytes))
	c.Request.ContentLength = int64(len(updatedBodyBytes))
	rp, err := proxy.NewReverseProxy(modelTarget.Target)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to create reverse proxy", slog.Any("error", err))
		c.String(http.StatusInternalServerError, fmt.Errorf("failed to create reverse proxy:%w", err).Error())
		return
	}

	var modComponent component.Moderation = nil
	isCheck, result, err := h.checkSensitive(c.Request.Context(), modelTarget.Model, chatReq, userUUID, chatReq.Stream)
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

	slog.InfoContext(c.Request.Context(), "proxy chat request to model target", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host),
		slog.Any("user", username), slog.Any("model_name", modelTarget.ModelName))

	tokenCounter := h.tokenCounterFactory.NewChat(token.CreateParam{
		Endpoint: modelTarget.Target,
		Host:     modelTarget.Host,
		Model:    modelTarget.ModelName,
		ImageID:  modelTarget.Model.ImageID,
		Provider: modelTarget.Model.Provider,
	})

	logCapture, err := component.NewLLMLogRecorder(
		trace.GetTraceIDInGinContext(c),
		modelTarget.ModelName,
		userUUID,
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
		slog.WarnContext(c.Request.Context(), "failed to initialize llmlog training capture", slog.Any("error", err))
	}

	w := NewResponseWriterWrapper(c.Writer, chatReq.Stream, modComponent, tokenCounter, logCapture)
	defer w.ClearBuffer()

	tokenCounter.AppendPrompts(chatReq.Messages)

	proxyToApi := ""
	if modelTarget.Model.Endpoint != "" {
		uri, err := url.ParseRequestURI(modelTarget.Model.Endpoint)
		if err != nil {
			slog.Warn("endpoint has wrong struct", slog.String("model", modelTarget.ModelName))
		} else {
			proxyToApi = uri.Path
		}
	}

	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(c.Request.Context(), "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}

	rp.ServeHTTP(w, c.Request, proxyToApi, modelTarget.Host)

	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		err := h.openaiComponent.RecordUsage(usageCtx, userUUID, modelTarget.Model, tokenCounter)
		if err != nil {
			slog.ErrorContext(usageCtx, "failed to record token usage", slog.Any("error", err))
		}
	}()

	go func() {
		if !h.config.AIGateway.EnableLLMLog || logCapture == nil || h.llmLogPublisher == nil {
			return
		}
		logCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		record, recordErr := logCapture.Record()
		if recordErr != nil {
			slog.ErrorContext(logCtx, "failed to build llmlog training record", slog.Any("error", recordErr))
			return
		}
		payload, marshalErr := json.Marshal(record)
		if marshalErr != nil {
			slog.ErrorContext(logCtx, "failed to marshal llmlog training record", slog.Any("error", marshalErr))
			return
		}
		if publishErr := h.llmLogPublisher.PublishTrainingLog(payload); publishErr != nil {
			slog.ErrorContext(logCtx, "failed to publish llmlog training record", slog.Any("error", publishErr))
		}
	}()
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
	userUUID := httpbase.GetCurrentUserUUID(c)
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
	modelTarget, err := h.resolveModelTarget(ctx, username, modelID)
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

	if err := h.openaiComponent.CheckBalance(ctx, username, userUUID); err != nil {
		h.handleInsufficientBalance(c, false, username, modelID, err)
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

	result, err := h.modComponent.CheckImagePrompts(ctx, req.Prompt, userUUID)
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
		if err := h.openaiComponent.RecordUsage(usageCtx, userUUID, modelTarget.Model, imageCounter); err != nil {
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
	userUUID := httpbase.GetCurrentUserUUID(c)
	modelTarget, err := h.resolveModelTarget(c.Request.Context(), username, modelID)
	if err != nil {
		handleModelTargetError(c, c.Request.Context(), modelID, "failed to get embedding target address", err)
		return
	}

	// Check balance before processing request
	if err := h.openaiComponent.CheckBalance(c.Request.Context(), username, userUUID); err != nil {
		h.handleInsufficientBalance(c, false, username, modelID, err)
		return
	}

	req.Model = modelTarget.ModelName
	data, _ := json.Marshal(req)
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Request.ContentLength = int64(len(data))
	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(c.Request.Context(), "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}
	slog.InfoContext(c, "proxy embedding request to model endpoint", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host),
		slog.Any("user", username), slog.Any("model_id", modelID))
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

		err := h.openaiComponent.RecordUsage(usageCtx, userUUID, modelTarget.Model, tokenCounter)
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
	userUUID := httpbase.GetCurrentUserUUID(c)
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

	modelTarget, err := h.resolveModelTarget(ctx, username, modelID)
	if err != nil {
		handleModelTargetError(c, ctx, modelID, "failed to get transcription target address", err)
		return
	}

	if !modelTarget.Model.SkipBalance() {
		if err := h.openaiComponent.CheckBalance(ctx, username, userUUID); err != nil {
			h.handleInsufficientBalance(c, false, username, modelID, err)
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

	slog.InfoContext(ctx, "proxy audio transcription request to model endpoint", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host),
		slog.Any("user", username), slog.Any("model_id", modelID), slog.Any("model_name", modelTarget.ModelName))

	audioCounter := token.NewAudioUsageCounter(token.NewTokenizerImpl(modelTarget.Target, modelTarget.Host, modelTarget.ModelName, modelTarget.Model.ImageID, modelTarget.Model.Provider))
	w := NewResponseWriterWrapperAudio(c.Writer, audioCounter)
	rp.ServeHTTP(w, c.Request, proxyToApi, modelTarget.Host)

	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		if err := h.openaiComponent.RecordUsage(usageCtx, userUUID, modelTarget.Model, audioCounter); err != nil {
			slog.ErrorContext(usageCtx, "failed to record audio transcription usage", slog.Any("error", err))
		}
	}()
}
