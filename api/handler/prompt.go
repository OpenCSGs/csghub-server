package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type PromptHandler struct {
	pc *component.PromptComponent
	sc component.SensitiveChecker
}

func NewPromptHandler(cfg *config.Config) (*PromptHandler, error) {
	promptComp, err := component.NewPromptComponent(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create PromptComponent: %w", err)
	}
	return &PromptHandler{
		pc: promptComp,
		sc: component.NewSensitiveComponent(cfg),
	}, nil
}

// GetVisiablePrompts godoc
// @Security     ApiKey
// @Summary      Get Visiable Prompt repos for current user
// @Description  get visiable Prompt repos for current user
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user"
// @Param        search query string false "search text"
// @Param        task_tag query string false "filter by task tag"
// @Param        framework_tag query string false "filter by framework tag"
// @Param        license_tag query string false "filter by license tag"
// @Param        language_tag query string false "filter by language tag"
// @Param        sort query string false "sort by"
// @Param        source query string false "source" Enums(opencsg, huggingface, local)
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.PromptRes,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts [get]
func (h *PromptHandler) Index(ctx *gin.Context) {
	filter := new(types.RepoFilter)
	filter.Tags = parseTagReqs(ctx)
	filter.Username = httpbase.GetCurrentUser(ctx)
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format prompt list", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filter = getFilterFromContext(ctx, filter)
	if !slices.Contains[[]string](Sorts, filter.Sort) {
		msg := fmt.Sprintf("sort parameter must be one of %v", Sorts)
		slog.Error("Bad request format,", slog.String("error", msg))
		ctx.JSON(http.StatusBadRequest, gin.H{"message": msg})
		return
	}

	if filter.Source != "" && !slices.Contains[[]string](Sources, filter.Source) {
		msg := fmt.Sprintf("source parameter must be one of %v", Sources)
		slog.Error("Bad request format,", slog.String("error", msg))
		ctx.JSON(http.StatusBadRequest, gin.H{"message": msg})
		return
	}

	prompts, total, err := h.pc.IndexPromptRepo(ctx, filter, per, page)
	if err != nil {
		slog.Error("Failed to get prompts dataset", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  prompts,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// ListPrompt    godoc
// @Security     ApiKey
// @Summary      List prompts
// @Description  List prompts
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param	     namespace path  string  true  "namespace"
// @Param		 name path  string  true  "name"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name} [get]
func (h *PromptHandler) ListPrompt(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	detail, err := h.pc.Show(ctx, namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		slog.Error("Failed to get prompt detail", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	req := types.PromptReq{
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
	}
	data, err := h.pc.ListPrompt(ctx, req)
	if err != nil {
		slog.Error("Failed to list prompts of repo", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"detail":  detail,
		"prompts": data,
	}
	httpbase.OK(ctx, respData)
}

// GetPrompt     godoc
// @Security     ApiKey
// @Summary      Get prompts by file
// @Description  Get prompts by file
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param	     namespace path  string  true  "namespace"
// @Param		 name path  string  true  "name"
// @Param		 file_path path string true "the file relative path"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name}/prompt/view/{file_path} [get]
func (h *PromptHandler) GetPrompt(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filePath := ctx.Param("file_path")
	if filePath == "" {
		slog.Error("Bad request file path format", "error", err)
		httpbase.BadRequest(ctx, "Bad request file path format")
		return
	}
	req := types.PromptReq{
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		Path:        convertFilePathFromRoute(filePath),
	}
	data, err := h.pc.GetPrompt(ctx, req)
	if err != nil {
		slog.Error("Failed to list prompts of repo", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, data)
}

// CreatePrompt  godoc
// @Security     ApiKey
// @Summary      Create prompt in repo
// @Description  Create prompt in repo
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param	     namespace path  string  true  "namespace"
// @Param		 name path  string  true  "name"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name}/prompt/file [post]
func (h *PromptHandler) CreatePrompt(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var body *component.CreatePromptReq
	if err := ctx.ShouldBindJSON(&body); err != nil {
		slog.Error("Bad request prompt format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	_, err = h.sc.CheckRequestV2(ctx, body)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	req := types.PromptReq{
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
	}

	data, err := h.pc.CreatePrompt(ctx, req, body)
	if err != nil {
		slog.Error("Failed to create prompt file of repo", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, data)
}

// UpdatePrompt  godoc
// @Security     ApiKey
// @Summary      Update prompt in repo
// @Description  Update prompt in repo
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param	     namespace path  string  true  "namespace"
// @Param		 name path  string  true  "name"
// @Param		 file_path path string true "the file relative path"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name}/prompt/file/{file_path} [put]
func (h *PromptHandler) UpdatePrompt(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filePath := ctx.Param("file_path")
	if filePath == "" {
		slog.Error("Bad request file path format", "error", err)
		httpbase.BadRequest(ctx, "Bad request file path format")
		return
	}

	var body *component.UpdatePromptReq
	if err := ctx.ShouldBindJSON(&body); err != nil {
		slog.Error("Bad request prompt format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	_, err = h.sc.CheckRequestV2(ctx, body)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	req := types.PromptReq{
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		Path:        convertFilePathFromRoute(filePath),
	}
	data, err := h.pc.UpdatePrompt(ctx, req, body)
	if err != nil {
		slog.Error("Failed to update prompt file of repo", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, data)
}

// DeletePrompt  godoc
// @Security     ApiKey
// @Summary      Delete prompt in repo
// @Description  Delete prompt in repo
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param	     namespace path  string  true  "namespace"
// @Param		 name path  string  true  "name"
// @Param		 file_path path string true "the file relative path"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name}/prompt/file/{file_path} [delete]
func (h *PromptHandler) DeletePrompt(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	filePath := ctx.Param("file_path")
	if filePath == "" {
		slog.Error("Bad request file path format", "error", err)
		httpbase.BadRequest(ctx, "Bad request file path format")
		return
	}

	req := types.PromptReq{
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		Path:        convertFilePathFromRoute(filePath),
	}
	err = h.pc.DeletePrompt(ctx, req)
	if err != nil {
		slog.Error("Failed to remove prompt file of repo", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// NewConversation godoc
// @Security     ApiKey
// @Summary      Create new conversation
// @Description  Create new conversation
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        body body types.Conversation true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/conversations [post]
func (h *PromptHandler) NewConversation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var body *types.ConversationTitle
	if err := ctx.ShouldBindJSON(&body); err != nil {
		slog.Error("Bad request conversation body", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req := types.ConversationTitleReq{
		CurrentUser: currentUser,
		ConversationTitle: types.ConversationTitle{
			Uuid:  body.Uuid,
			Title: body.Title,
		},
	}
	resp, err := h.pc.NewConversation(ctx, req)
	if err != nil {
		slog.Error("Failed to create conversation", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, resp)
}

// ListConversation godoc
// @Security     ApiKey
// @Summary      List conversations of user
// @Description  List conversations of user
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/conversations [get]
func (h *PromptHandler) ListConversation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	data, err := h.pc.ListConversationsByUserID(ctx, currentUser)
	if err != nil {
		slog.Error("Failed to list conversations", slog.Any("currentUser", currentUser), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, data)
}

// GetConversation godoc
// @Security     ApiKey
// @Summary      Get a conversation by uuid
// @Description  Get a conversation by uuid
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        id   path string true "conversation uuid"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/conversations/{id} [get]
func (h *PromptHandler) GetConversation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.Error("Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	req := types.ConversationReq{
		CurrentUser: currentUser,
		Conversation: types.Conversation{
			Uuid: uuid,
		},
	}
	conversation, err := h.pc.GetConversation(ctx, req)
	if err != nil {
		slog.Error("Failed to get conversation by id", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, conversation)
}

// SubmitMessage godoc
// @Security     ApiKey
// @Summary      Submit a conversation message
// @Description  Submit a conversation message
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        id   path string true "conversation uuid"
// @Param        body body types.Conversation true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/conversations/{id} [post]
func (h *PromptHandler) SubmitMessage(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.Error("Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	var body *types.Conversation
	if err := ctx.ShouldBindJSON(&body); err != nil {
		slog.Error("Bad request messsage body", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req := types.ConversationReq{
		CurrentUser: currentUser,
		Conversation: types.Conversation{
			Uuid:        uuid,
			Message:     body.Message,
			Temperature: body.Temperature,
		},
	}

	ch, err := h.pc.SubmitMessage(ctx, req)
	if err != nil {
		slog.Error("Failed to submit message", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	generatedText := ""
	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("generate respose end for context done", slog.Any("error", ctx.Request.Context().Err()))
			res := types.Conversation{
				Uuid:    uuid,
				Message: generatedText,
			}
			_, err = h.pc.SaveGeneratedText(ctx, res)
			if err != nil {
				slog.Error("fail to save generated message for request cancel", slog.Any("res", res), slog.Any("error", err))
				httpbase.ServerError(ctx, err)
			}
			return
		case data, ok := <-ch:
			if ok {
				if len(data) < 1 {
					continue
				}
				data := strings.TrimSpace(strings.TrimPrefix(data, "data:"))
				ctx.SSEvent("data", data)
				ctx.Writer.Flush()
				resp := types.LLMResponse{}
				err := json.Unmarshal([]byte(data), &resp)
				if err != nil {
					slog.Warn("unmarshal llm response", slog.Any("data", data), slog.Any("error", err))
					continue
				}
				if len(resp.Choices) < 1 {
					continue
				}
				generatedText = fmt.Sprintf("%s%s", generatedText, resp.Choices[0].Delta.Content)
			} else {
				slog.Debug("stream channel closed")
				res := types.Conversation{
					Uuid:    uuid,
					Message: generatedText,
				}
				msg, err := h.pc.SaveGeneratedText(ctx, res)
				if err != nil {
					slog.Error("fail to save generated message for stream close", slog.Any("res", res), slog.Any("error", err))
					httpbase.ServerError(ctx, err)
				}
				ctx.SSEvent("data", fmt.Sprintf("{\"msg_id\": %d}", msg.ID))
				ctx.Writer.Flush()
				return
			}
		}
	}
}

// UpdateConversation godoc
// @Security     ApiKey
// @Summary      Update a conversation title
// @Description  Update a conversation title
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        id   path string true "conversation uuid"
// @Param        body body types.ConversationTitle true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/conversations/{id} [put]
func (h *PromptHandler) UpdateConversation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.Error("Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	var body *types.ConversationTitle
	if err := ctx.ShouldBindJSON(&body); err != nil {
		slog.Error("Bad request messsage body", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req := types.ConversationTitleReq{
		CurrentUser: currentUser,
		ConversationTitle: types.ConversationTitle{
			Uuid:  uuid,
			Title: body.Title,
		},
	}
	resp, err := h.pc.UpdateConversation(ctx, req)
	if err != nil {
		slog.Error("Failed to update conversation", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, resp)
}

// DeleteConversation godoc
// @Security     ApiKey
// @Summary      Delete a conversation
// @Description  Delete a conversation
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        id   path string true  "conversation uuid"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/conversations/{id} [delete]
func (h *PromptHandler) RemoveConversation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.Error("Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	req := types.ConversationReq{
		CurrentUser: currentUser,
		Conversation: types.Conversation{
			Uuid: uuid,
		},
	}
	err := h.pc.RemoveConversation(ctx, req)
	if err != nil {
		slog.Error("Failed to remove conversation by id", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// LikeMessage godoc
// @Security     ApiKey
// @Summary      Like a conversation message
// @Description  Like a conversation message
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        uuid path string true  "conversation uuid"
// @Param        id  path string true  "message id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/conversations/{id}/message/{msgid}/like [put]
func (h *PromptHandler) LikeMessage(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.Error("Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	msgid := ctx.Param("msgid")
	idInt, err := strconv.ParseInt(msgid, 10, 64)
	if err != nil {
		slog.Error("Bad request message id", slog.Any("msgid", msgid), slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req := types.ConversationMessageReq{
		Uuid:        uuid,
		Id:          idInt,
		CurrentUser: currentUser,
	}
	err = h.pc.LikeConversationMessage(ctx, req)
	if err != nil {
		slog.Error("Failed to like conversation message", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// HateMessage godoc
// @Security     ApiKey
// @Summary      Hate a conversation message
// @Description  Hate a conversation message
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        uuid path string true  "conversation uuid"
// @Param        id  path string true  "message id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/conversations/{id}/message/{msgid}/hate [put]
func (h *PromptHandler) HateMessage(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.Error("Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	msgid := ctx.Param("msgid")
	idInt, err := strconv.ParseInt(msgid, 10, 64)
	if err != nil {
		slog.Error("Bad request message id", slog.Any("msgid", msgid), slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req := types.ConversationMessageReq{
		Uuid:        uuid,
		Id:          idInt,
		CurrentUser: currentUser,
	}
	err = h.pc.HateConversationMessage(ctx, req)
	if err != nil {
		slog.Error("Failed to hate conversation message", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// PromptRelations      godoc
// @Security     ApiKey
// @Summary      Get prompt related assets
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{data=types.Relations} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name}/relations [get]
func (h *PromptHandler) Relations(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	detail, err := h.pc.Relations(ctx, namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		slog.Error("Failed to get prompt relations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, detail)
}

// SetRelation   godoc
// @Security     ApiKey
// @Summary      Set model relation for prompt
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param        req body types.RelationModels true  "set model relation"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name}/relations [put]
func (h *PromptHandler) SetRelations(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var req types.RelationModels
	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Namespace = namespace
	req.Name = name
	req.CurrentUser = currentUser

	err = h.pc.SetRelationModels(ctx, req)
	if err != nil {
		slog.Error("Failed to set models for prompt", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// AddModelRelation   godoc
// @Security     ApiKey
// @Summary      add model relation for prompt
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param        req body types.RelationModel true  "add model relation"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name}/relations/model [post]
func (h *PromptHandler) AddModelRelation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var req types.RelationModel
	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Namespace = namespace
	req.Name = name
	req.CurrentUser = currentUser

	err = h.pc.AddRelationModel(ctx, req)
	if err != nil {
		slog.Error("Failed to add model for prompt", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// DeleteModelRelation  godoc
// @Security     ApiKey
// @Summary      delete model relation for prompt
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param        req body types.RelationModel true  "delelet model relation"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name}/relations/model [delete]
func (h *PromptHandler) DelModelRelation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var req types.RelationModel
	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Namespace = namespace
	req.Name = name
	req.CurrentUser = currentUser

	err = h.pc.DelRelationModel(ctx, req)
	if err != nil {
		slog.Error("Failed to delete dataset for model", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// CreatePromptRepo  godoc
// @Security     ApiKey
// @Summary      Create a new prompt repo
// @Description  create a new prompt repo
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user, the owner"
// @Param        body body types.CreatePromptRepoReq true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts [post]
func (h *PromptHandler) Create(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var req *types.CreatePromptRepoReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request prompt repo format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	_, err := h.sc.CheckRequest(ctx, req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}
	req.Username = currentUser

	prompt, err := h.pc.CreatePromptRepo(ctx, req)
	if err != nil {
		slog.Error("Failed to create prompt repo", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Create prompt repo succeed", slog.String("prompt", prompt.Name))
	respData := gin.H{
		"data": prompt,
	}
	ctx.JSON(http.StatusOK, respData)
}

// UpdatePromptRepo  godoc
// @Security     ApiKey
// @Summary      Update a exists prompt repo
// @Description  update a exists prompt repo
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user, the owner"
// @Param        body body types.UpdatePromptRepoReq true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name} [put]
func (h *PromptHandler) Update(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var req *types.UpdatePromptRepoReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	_, err := h.sc.CheckRequest(ctx, req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}
	req.Username = currentUser

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Namespace = namespace
	req.Name = name

	prompt, err := h.pc.UpdatePromptRepo(ctx, req)
	if err != nil {
		slog.Error("Failed to update prompt repo", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, prompt)
}

// DeletePromptRepo  godoc
// @Security     ApiKey
// @Summary      Delete a exists prompt repo
// @Description  delete a exists prompt repo
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user, the owner"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name} [delete]
func (h *PromptHandler) Delete(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = h.pc.RemoveRepo(ctx, namespace, name, currentUser)
	if err != nil {
		slog.Error("Failed to delete prompt repo", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// GetRepoBranches
// @Security     ApiKey
// @Summary      Get the branches of prompt repository
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 ref query string true "branch or tag"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Branch} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name}/branches [get]
func (h *PromptHandler) Branches(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.GetBranchesReq{
		Namespace:   namespace,
		Name:        name,
		Per:         per,
		Page:        page,
		RepoType:    types.PromptRepo,
		CurrentUser: currentUser,
	}
	branches, err := h.pc.Branches(ctx, req)
	if err != nil {
		slog.Error("Failed to get prompt repo branches", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, branches)
}

// GetRepoTags
// @Security     ApiKey
// @Summary      Get the tags of prompt repository
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 current_user query string false "current user name"
// @Success      200  {object}  types.ResponseWithTotal{data=[]database.Tag} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name}/tags [get]
func (h *PromptHandler) Tags(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.GetTagsReq{
		Namespace:   namespace,
		Name:        name,
		RepoType:    types.PromptRepo,
		CurrentUser: currentUser,
	}
	tags, err := h.pc.Tags(ctx, req)
	if err != nil {
		slog.Error("Failed to get prompt repo tags", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, tags)
}

// UpdateRepoTags
// @Security     ApiKey
// @Summary      update the tags of a certain category
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param		 namespace path string true "repo owner name"
// @Param		 name path string true "repo name"
// @Param		 current_user query string true "current user name"
// @Param		 category path string true "tag category" Enums(task, license, framework, language, industry)
// @Param		 tags body []string true "tag names in array"
// @Success      200  {object}  types.Response "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name}/tags/{category} [post]
func (h *PromptHandler) UpdateTags(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, httpbase.ErrorNeedLogin)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Failed update tags", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var tags []string
	if err := ctx.ShouldBindJSON(&tags); err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("failed to unmarshal tags: %w", err).Error())
		return
	}
	category := ctx.Param("category")

	err = h.pc.UpdateTags(ctx, namespace, name, types.PromptRepo, category, currentUser, tags)
	if err != nil {
		slog.Error("Failed to update tags", slog.String("error", err.Error()), slog.String("category", category), slog.String("namespace", namespace), slog.String("name", name))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

func (h *PromptHandler) UpdateDownloads(ctx *gin.Context) {
	var req *types.UpdateDownloadsReq
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.Namespace = namespace
	req.Name = name
	req.RepoType = types.PromptRepo
	date, err := time.Parse("2006-01-02", req.ReqDate)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Date = date

	err = h.pc.UpdateDownloads(ctx, req)
	if err != nil {
		slog.Error("Failed to update repo download count", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name), slog.Time("date", date), slog.Int64("clone_count", req.CloneCount))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}
