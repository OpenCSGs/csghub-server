package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type PromptOptimizeHandler interface {
	NewConversation(ctx *gin.Context)
	ListConversation(ctx *gin.Context)
	GetConversation(ctx *gin.Context)
	SubmitMessage(ctx *gin.Context)
	UpdateConversation(ctx *gin.Context)
	RemoveConversation(ctx *gin.Context)
	LikeMessage(ctx *gin.Context)
	HateMessage(ctx *gin.Context)
	SummarizeTitle(ctx *gin.Context)
}

type promptOptimizeHandlerImpl struct {
	prompt    component.PromptComponent
	sensitive component.SensitiveComponent
	repo      component.RepoComponent
}

func NewPromptOptimizeHandler(cfg *config.Config) (PromptOptimizeHandler, error) {
	promptComp, err := component.NewPromptComponent(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create PromptComponent: %w", err)
	}
	sc, err := component.NewSensitiveComponent(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create SensitiveComponent: %w", err)
	}
	repo, err := component.NewRepoComponent(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo component: %w", err)
	}
	return &promptOptimizeHandlerImpl{
		prompt:    promptComp,
		sensitive: sc,
		repo:      repo,
	}, nil
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
func (h *promptOptimizeHandlerImpl) NewConversation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var body *types.ConversationTitle
	if err := ctx.ShouldBindJSON(&body); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request conversation body", "error", err)
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
	resp, err := h.prompt.NewConversation(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create conversation", slog.Any("req", req), slog.Any("error", err))
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
func (h *promptOptimizeHandlerImpl) ListConversation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	data, err := h.prompt.ListConversationsByUserID(ctx.Request.Context(), currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to list conversations", slog.Any("currentUser", currentUser), slog.Any("error", err))
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
func (h *promptOptimizeHandlerImpl) GetConversation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.ErrorContext(ctx.Request.Context(), "Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	req := types.ConversationReq{
		CurrentUser: currentUser,
		Conversation: types.Conversation{
			Uuid: uuid,
		},
	}
	conversation, err := h.prompt.GetConversation(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get conversation by id", slog.Any("req", req), slog.Any("error", err))
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
func (h *promptOptimizeHandlerImpl) SubmitMessage(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.ErrorContext(ctx.Request.Context(), "Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	var body *types.Conversation
	if err := ctx.ShouldBindJSON(&body); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request messsage body", "error", err)
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

	ch, err := h.prompt.SubmitMessage(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to submit message", slog.Any("req", req), slog.Any("error", err))
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
			_, err = h.prompt.SaveGeneratedText(ctx.Request.Context(), res)
			if err != nil {
				slog.ErrorContext(ctx.Request.Context(), "fail to save generated message for request cancel", slog.Any("res", res), slog.Any("error", err))
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
				msg, err := h.prompt.SaveGeneratedText(ctx.Request.Context(), res)
				if err != nil {
					slog.ErrorContext(ctx.Request.Context(), "fail to save generated message for stream close", slog.Any("res", res), slog.Any("error", err))
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
func (h *promptOptimizeHandlerImpl) UpdateConversation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.ErrorContext(ctx.Request.Context(), "Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	var body *types.ConversationTitle
	if err := ctx.ShouldBindJSON(&body); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request messsage body", "error", err)
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
	resp, err := h.prompt.UpdateConversation(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update conversation", slog.Any("req", req), slog.Any("error", err))
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
func (h *promptOptimizeHandlerImpl) RemoveConversation(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.ErrorContext(ctx.Request.Context(), "Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	req := types.ConversationReq{
		CurrentUser: currentUser,
		Conversation: types.Conversation{
			Uuid: uuid,
		},
	}
	err := h.prompt.RemoveConversation(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to remove conversation by id", slog.Any("req", req), slog.Any("error", err))
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
func (h *promptOptimizeHandlerImpl) LikeMessage(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.ErrorContext(ctx.Request.Context(), "Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	msgid := ctx.Param("msgid")
	idInt, err := strconv.ParseInt(msgid, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request message id", slog.Any("msgid", msgid), slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req := types.ConversationMessageReq{
		Uuid:        uuid,
		Id:          idInt,
		CurrentUser: currentUser,
	}
	err = h.prompt.LikeConversationMessage(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to like conversation message", slog.Any("req", req), slog.Any("error", err))
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
func (h *promptOptimizeHandlerImpl) HateMessage(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.ErrorContext(ctx.Request.Context(), "Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	msgid := ctx.Param("msgid")
	idInt, err := strconv.ParseInt(msgid, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request message id", slog.Any("msgid", msgid), slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req := types.ConversationMessageReq{
		Uuid:        uuid,
		Id:          idInt,
		CurrentUser: currentUser,
	}
	err = h.prompt.HateConversationMessage(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to hate conversation message", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// SummarizeTitle godoc
// @Security     ApiKey
// @Summary      Summarize a conversation title
// @Description  Summarize a conversation title
// @Tags         Prompt
// @Accept       json
// @Produce      json
// @Param        id   path string true "conversation uuid"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /prompts/conversations/{id}/summary [put]
func (h *promptOptimizeHandlerImpl) SummarizeTitle(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	uuid := ctx.Param("id")
	if len(uuid) < 1 {
		slog.ErrorContext(ctx.Request.Context(), "Bad request conversation uuid")
		httpbase.BadRequest(ctx, "uuid is empty")
		return
	}
	req := types.ConversationTitleReq{
		CurrentUser: currentUser,
		ConversationTitle: types.ConversationTitle{
			Uuid: uuid,
		},
	}

	resp, err := h.prompt.SummarizeConversationTitle(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to summarize conversation title", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, resp)
}
