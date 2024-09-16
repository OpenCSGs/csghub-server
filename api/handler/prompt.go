package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type PromptHandler struct {
	c  *component.DatasetComponent
	pc *component.PromptComponent
}

func NewPromptHandler(cfg *config.Config) (*PromptHandler, error) {
	dsComp, err := component.NewDatasetComponent(cfg)
	if err != nil {
		return nil, err
	}
	promptComp, err := component.NewPromptComponent(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create PromptComponent: %w", err)
	}
	return &PromptHandler{
		pc: promptComp,
		c:  dsComp,
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
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Dataset,total=int} "OK"
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

	prompts, total, err := h.c.IndexPrompt(ctx, filter, per, page)
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
	req := types.PromptReq{
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
	}
	data, err := h.pc.ListPrompt(ctx, req)
	if err != nil {
		slog.Error("Failed to list prompts of repo", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
	}
	httpbase.OK(ctx, data)
}

// GetPrompt     godoc
// @Security     ApiKey
// @Summary      Get prompts by file
// @Description  Get prompts by file
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/{namespace}/{name}/{file_path} [get]
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
		Path:        filePath,
	}
	data, err := h.pc.GetPrompt(ctx, req)
	if err != nil {
		slog.Error("Failed to list prompts of repo", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
	}
	httpbase.OK(ctx, data)
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
			err = h.pc.SaveGeneratedText(ctx, res)
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
				err = h.pc.SaveGeneratedText(ctx, res)
				if err != nil {
					slog.Error("fail to save generated message for stream close", slog.Any("res", res), slog.Any("error", err))
					httpbase.ServerError(ctx, err)
				}
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
