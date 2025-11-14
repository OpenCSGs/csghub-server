package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
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
	if config.SensitiveCheck.Enable {
		modSvcClient = rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", config.Moderation.Host, config.Moderation.Port))
	}
	cacheClient, err := cache.NewCache(context.Background(), cache.RedisConfig{
		Addr:     config.Redis.Endpoint,
		Username: config.Redis.User,
		Password: config.Redis.Password,
	})
	if err != nil {
		return nil, err
	}
	modComponent := component.NewModerationImplWithClient(modSvcClient, cacheClient)
	return newOpenAIHandler(modelService, repoComp, modSvcClient, modComponent), nil
}

func newOpenAIHandler(
	modelService component.OpenAIComponent,
	repoComp apicomp.RepoComponent,
	modSvcClient rpc.ModerationSvcClient,
	modComponent component.Moderation,
) *OpenAIHandlerImpl {
	return &OpenAIHandlerImpl{
		openaiComponent: modelService,
		repoComp:        repoComp,
		modSvcClient:    modSvcClient,
		modComponent:    modComponent,
	}
}

// OpenAIHandlerImpl implements the OpenAIHandler interface
type OpenAIHandlerImpl struct {
	openaiComponent component.OpenAIComponent
	repoComp        apicomp.RepoComponent
	modSvcClient    rpc.ModerationSvcClient
	modComponent    component.Moderation
}

// ListModels godoc
// @Security     ApiKey
// @Summary      List available models
// @Description  Returns a list of available models
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.ModelList "OK"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/models [get]
func (h *OpenAIHandlerImpl) ListModels(c *gin.Context) {
	currentUser := httpbase.GetCurrentUser(c)
	models, err := h.openaiComponent.GetAvailableModels(c.Request.Context(), currentUser)
	if err != nil {
		slog.Error("failed to get available models", "error", err.Error(), "current_user", currentUser)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": types.Error{
				Code:    "internal_server_error",
				Message: "Failed to retrieve models",
				Type:    "server_error",
			}})
		return
	}

	response := types.ModelList{
		Object: "list",
		Data:   models,
	}

	c.PureJSON(http.StatusOK, response)
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

// Chat godoc
// @Security     ApiKey
// @Summary      Chat with backend model
// @Description  Sends a chat completion request to the backend model and returns the response
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request body ChatCompletionRequest true "Chat completion request"
// @Success      200  {object}  ChatCompletionResponse "OK"
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
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		slog.Error("failed to read request body", "error", err.Error())
		c.String(http.StatusBadRequest, fmt.Errorf("invalid chat compoletion request body:%w", err).Error())
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.Request.ContentLength = int64(len(bodyBytes))

	if err := json.Unmarshal(bodyBytes, chatReq); err != nil {
		slog.Error("failed to parse request body", "error", err.Error())
		c.String(http.StatusBadRequest, fmt.Errorf("invalid chat compoletion request body:%w", err).Error())
		return
	}

	validate := validator.New()
	err = validate.Struct(chatReq)
	if err != nil {
		slog.Error("invalid chat compoletion request body", "error", err.Error())
		c.String(http.StatusBadRequest, fmt.Errorf("invalid chat compoletion request body:%w", err).Error())
		return
	}
	modelID := chatReq.Model
	model, err := h.openaiComponent.GetModelByID(c.Request.Context(), username, modelID)
	if err != nil {
		slog.Error("failed to get model by id", "model_id", modelID, "error", err.Error())
		c.String(http.StatusInternalServerError, fmt.Errorf("failed to get model by id '%s',error:%w", modelID, err).Error())
		return
	}
	if model == nil {
		slog.Error("model not found", "model_id", modelID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": types.Error{
				Code:    "model_not_found",
				Message: fmt.Sprintf("model '%s' not found", modelID),
				Type:    "invalid_request_error",
			}})
		return
	}

	endpoint := model.Endpoint
	if endpoint == "" {
		slog.Error("model not running, endpoint is empty", "model_id", modelID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": types.Error{
				Code:    "model_not_running",
				Message: fmt.Sprintf("model '%s' not running", modelID),
				Type:    "invalid_request_error",
			}})
		return
	}

	modelName, _, err := (component.ModelIDBuilder{}).From(modelID)
	if err != nil {
		slog.Error("failed to process chat request", "error", err, "model_id", modelID)
		c.String(http.StatusBadRequest, err.Error())
		return
	}

	var reqMap map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqMap); err != nil {
		slog.Error("failed to unmarshal request body to map", "error", err)
		c.String(http.StatusBadRequest, fmt.Errorf("invalid chat completion request body: %w", err).Error())
		return
	}
	// directly update model field in request map
	reqMap["model"] = modelName

	if chatReq.Stream {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		if !strings.Contains(model.ImageID, "vllm-cpu") {
			chatReq.StreamOptions = &StreamOptions{
				IncludeUsage: true,
			}
		}
	}

	// marshal updated request map back to JSON bytes
	updatedBodyBytes, err := json.Marshal(reqMap)
	if err != nil {
		slog.Error("failed to marshal updated request map", "error", err)
		c.String(http.StatusInternalServerError, fmt.Errorf("failed to process chat request: %w", err).Error())
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(updatedBodyBytes))
	c.Request.ContentLength = int64(len(updatedBodyBytes))
	rp, _ := proxy.NewReverseProxy(endpoint)
	slog.Info("proxy chat request to model endpoint", "endpoint", endpoint, "user", username, "model_name", modelName)
	w := NewResponseWriterWrapper(c.Writer, chatReq.Stream)
	if h.modSvcClient != nil {
		w.WithModeration(h.modSvcClient)
	}
	tokenizer := token.NewTokenizerImpl(endpoint, modelName, model.ImageID)
	llmTokenCounter := token.NewLLMTokenCounter(tokenizer)
	for _, msg := range chatReq.Messages {
		if h.modSvcClient != nil {
			content := msg.GetContent().AsAny()
			msgContent, ok := content.(*string)
			if !ok {
				break
			}
			result, err := h.modComponent.CheckLLMPrompt(c.Request.Context(), *msgContent, userUUID+modelID)
			if err != nil {
				c.String(http.StatusInternalServerError, fmt.Errorf("failed to call moderation error:%w", err).Error())
				return
			}
			if result.IsSensitive {
				slog.Debug("sensitive content", slog.String("reason", result.Reason))
				errorChunk := w.generateSensitiveRespForPrompt()
				errorChunkJson, _ := json.Marshal(errorChunk)
				_, err := c.Writer.Write([]byte("data: " + string(errorChunkJson) + "\n\n" + "[DONE]"))
				if err != nil {
					slog.Error("write into resp error:", slog.String("err", err.Error()))
				}
				c.Writer.Flush()
				return
			}
		}
		llmTokenCounter.AppendPrompts(types.Message{
			Role:    *msg.GetRole(),
			Content: msg.GetContent().AsAny().(*string),
		})
	}
	w.WithLLMTokenCounter(llmTokenCounter)

	rp.ServeHTTP(w, c.Request, "", "")

	go func() {
		err := h.openaiComponent.RecordUsage(c.Request.Context(), userUUID, model, llmTokenCounter)
		if err != nil {
			slog.Error("failed to record token usage", "error", err)
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
	if req.Input == "" || req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model and input cannot be empty"})
		return
	}
	modelID := req.Model
	username := httpbase.GetCurrentUser(c)
	userUUID := httpbase.GetCurrentUserUUID(c)
	model, err := h.openaiComponent.GetModelByID(c.Request.Context(), username, modelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	endpoint := model.Endpoint
	if endpoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": types.Error{
				Code:    "model_not_running",
				Message: fmt.Sprintf("model '%s' not running", modelID),
				Type:    "invalid_request_error",
			}})
		return
	}
	modelName, _, err := (component.ModelIDBuilder{}).From(modelID)
	if err != nil {
		slog.Error("failed to process chat request", "error", err, "model_id", modelID)
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	req.Model = modelName
	data, _ := json.Marshal(req)
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Request.ContentLength = int64(len(data))
	rp, _ := proxy.NewReverseProxy(endpoint)
	w := NewResponseWriterWrapperEmbedding(c.Writer)
	//TODO: use real tokenizer to count token usag
	if h.modSvcClient != nil {
		w.WithModeration(h.modSvcClient)
		result, err := h.modSvcClient.PassTextCheck(c, string(sensitive.ScenarioChatDetection), req.Input)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Errorf("failed to call moderation error:%w", err).Error())
			return
		}
		if result.IsSensitive {
			slog.Debug("sensitive content", slog.String("reason", result.Reason))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Input data may contain inappropriate content."})
			return
		}
	}

	tokenizer := token.NewTokenizerImpl(endpoint, modelName, model.ImageID)
	tokenCounter := token.NewEmbeddingTokenCounter(tokenizer)
	tokenCounter.Input(req.Input)
	w.WithTokenCounter(tokenCounter)
	slog.Info("proxy embedding request to model endpoint", "endpoint", endpoint, "user", username, "model_id", modelID)
	rp.ServeHTTP(w, c.Request, "", "")
	go func() {
		err := h.openaiComponent.RecordUsage(c.Request.Context(), userUUID, model, tokenCounter)
		if err != nil {
			slog.Error("failed to record embedding token usage", "error", err)
		}
	}()
}
