package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/cache"
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
	modComponent := component.NewModerationImplWithClient(modSvcClient, cacheClient)
	clusterComp, err := apicomp.NewClusterComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster component, error: %w", err)
	}
	return newOpenAIHandler(modelService, repoComp, modComponent, clusterComp, token.NewCounterFactory()), nil
}

func newOpenAIHandler(
	modelService component.OpenAIComponent,
	repoComp apicomp.RepoComponent,
	modComponent component.Moderation,
	clusterComp apicomp.ClusterComponent,
	tokenCounterFactory token.CounterFactory,
) *OpenAIHandlerImpl {
	return &OpenAIHandlerImpl{
		openaiComponent:     modelService,
		repoComp:            repoComp,
		modComponent:        modComponent,
		clusterComp:         clusterComp,
		tokenCounterFactory: tokenCounterFactory,
	}
}

// OpenAIHandlerImpl implements the OpenAIHandler interface
type OpenAIHandlerImpl struct {
	openaiComponent     component.OpenAIComponent
	repoComp            apicomp.RepoComponent
	modComponent        component.Moderation
	clusterComp         apicomp.ClusterComponent
	tokenCounterFactory token.CounterFactory
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

	if err = json.Unmarshal(bodyBytes, chatReq); err != nil {
		slog.Error("failed to parse request body", "error", err.Error())
		c.String(http.StatusBadRequest, fmt.Errorf("invalid chat compoletion request body:%w", err).Error())
		return
	}

	validate := validator.New()
	if err = validate.Struct(chatReq); err != nil {
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

	targetReq := commonType.EndpointReq{
		ClusterID: model.ClusterID,
		Target:    model.Endpoint,
		Host:      "",
		Endpoint:  model.Endpoint,
		SvcName:   model.SvcName,
	}
	target := ""
	host := ""
	if len(model.SvcName) > 0 {
		target, host, err = apicomp.ExtractDeployTargetAndHost(c.Request.Context(), h.clusterComp, targetReq)
	} else {
		slog.Debug("external model", slog.Any("model", model))
		target = model.Endpoint
	}
	if err != nil || len(target) < 1 {
		slog.Error("failed to get model target address", slog.Any("error", err),
			slog.Any("model", model), slog.Any("targetReq", targetReq), slog.Any("model_id", modelID),
			slog.Any("target", target), slog.Any("host", host))
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
	if err = json.Unmarshal(bodyBytes, &reqMap); err != nil {
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
	rp, _ := proxy.NewReverseProxy(target)
	slog.Info("proxy chat request to model target", slog.Any("target", target), slog.Any("host", host),
		slog.Any("user", username), slog.Any("model_name", modelName))
	// Create a combined key using userUUID and modelID for caching and tracking
	key := fmt.Sprintf("%s:%s", userUUID, modelID)
	result, err := h.modComponent.CheckChatPrompts(c.Request.Context(), chatReq.Messages, key)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Errorf("failed to call moderation error:%w", err).Error())
		return
	}
	if result.IsSensitive {
		slog.Debug("sensitive content", slog.String("reason", result.Reason))
		errorChunk := generateSensitiveRespForPrompt()
		errorChunkJson, _ := json.Marshal(errorChunk)
		_, err := c.Writer.Write([]byte("data: " + string(errorChunkJson) + "\n\n" + "[DONE]"))
		if err != nil {
			slog.Error("write into resp error:", slog.String("err", err.Error()))
		}
		c.Writer.Flush()
		return
	}
	tokenCounter := h.tokenCounterFactory.NewChat(token.CreateParam{
		Endpoint: target,
		Host:     host,
		Model:    modelName,
		ImageID:  model.ImageID,
	})
	w := NewResponseWriterWrapper(c.Writer, chatReq.Stream, h.modComponent, tokenCounter)
	defer w.ClearBuffer()

	tokenCounter.AppendPrompts(chatReq.Messages)

	proxyToApi := ""
	if model.Endpoint != "" {
		uri, err := url.ParseRequestURI(model.Endpoint)
		if err != nil {
			slog.Warn("endpoint has wrong struct ", slog.String("model", modelName))
		} else {
			proxyToApi = uri.Path
		}
	}

	if model.AuthHead != "" {
		var authMap map[string]string
		if err := json.Unmarshal([]byte(model.AuthHead), &authMap); err != nil {
			slog.Warn("invalid auth head", slog.String("model", modelName))
		} else {
			for authKey, authVal := range authMap {
				c.Request.Header.Set(authKey, authVal)
			}
		}
	}

	rp.ServeHTTP(w, c.Request, proxyToApi, host)

	go func() {
		err := h.openaiComponent.RecordUsage(c.Request.Context(), userUUID, model, tokenCounter)
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

	targetReq := commonType.EndpointReq{
		ClusterID: model.ClusterID,
		Target:    model.Endpoint,
		Host:      "",
		Endpoint:  model.Endpoint,
		SvcName:   model.SvcName,
	}
	target := ""
	host := ""
	if len(model.SvcName) > 0 {
		target, host, err = apicomp.ExtractDeployTargetAndHost(c.Request.Context(), h.clusterComp, targetReq)
	} else {
		target = model.Endpoint
	}
	if err != nil || len(target) < 1 {
		slog.ErrorContext(c, "failed to get embedding target address", slog.Any("error", err),
			slog.Any("model", model), slog.Any("targetReq", targetReq), slog.Any("model_id", modelID),
			slog.Any("target", target), slog.Any("host", host))
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
		slog.ErrorContext(c, "failed to process chat request", "error", err, "model_id", modelID)
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	req.Model = modelName
	data, _ := json.Marshal(req)
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Request.ContentLength = int64(len(data))
	slog.InfoContext(c, "proxy embedding request to model endpoint", slog.Any("target", target), slog.Any("host", host),
		slog.Any("user", username), slog.Any("model_id", modelID))
	rp, _ := proxy.NewReverseProxy(target)
	w := NewResponseWriterWrapperEmbedding(c.Writer)

	tokenCounter := h.tokenCounterFactory.NewEmbedding(token.CreateParam{
		Endpoint: target,
		Host:     host,
		Model:    modelName,
		ImageID:  model.ImageID,
	})
	tokenCounter.Input(req.Input)

	rp.ServeHTTP(w, c.Request, "", host)
	go func() {
		err := h.openaiComponent.RecordUsage(c.Request.Context(), userUUID, model, tokenCounter)
		if err != nil {
			slog.ErrorContext(c, "failed to record embedding token usage", "error", err)
		}
	}()
}
