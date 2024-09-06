package handler

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type DiscussionHandler struct {
	c *component.DiscussionComponent
}

func NewDiscussionHandler() (*DiscussionHandler, error) {
	c := component.NewDiscussionComponent()
	return &DiscussionHandler{
		c: c,
	}, nil
}

// CreateRepoDiscussion godoc
// @Security     ApiKey
// @Summary      Create a new repo discussion
// @Description  create a new repo discussion
// @Tags         Discussion
// @Accept       json
// @Produce      json
// @Param        current_user query string true "current user, the owner"
// @Param        repo_type path string true "repository type" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        body body component.CreateRepoDiscussionRequest true "body"
// @Success      200  {object}  types.Response{data=component.CreateDiscussionResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/discussions [post]
func (h *DiscussionHandler) CreateRepoDiscussion(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	repoType := h.getRepoType(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var req component.CreateRepoDiscussionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.CurrentUser = currentUser
	req.RepoType = types.RepositoryType(repoType)
	req.Namespace = namespace
	req.Name = name
	resp, err := h.c.CreateRepoDiscussion(ctx, req)
	if err != nil {
		slog.Error("Failed to create repo discussion", "error", err, "request", req)
		httpbase.ServerError(ctx, fmt.Errorf("failed to create repo discussion: %w", err))
		return
	}
	httpbase.OK(ctx, resp)
}

// UpdateDiscussion godoc
// @Security     ApiKey
// @Summary      Update a discussion
// @Description  update a discussion
// @Tags         Discussion
// @Accept       json
// @Produce      json
// @Param        id path string true "the discussion id"
// @Param        current_user query string true "current user, the owner"
// @Param        body body component.UpdateDiscussionRequest true "body"
// @Success      200  {object}  types.Response "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /discussions/{id} [put]
func (h *DiscussionHandler) UpdateDiscussion(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, "invalid discussion id:"+id)
		return
	}
	var req component.UpdateDiscussionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.ID = idInt
	req.CurrentUser = currentUser
	err = h.c.UpdateDiscussion(ctx, req)
	if err != nil {
		slog.Error("Failed to update discussion", "error", err, "request", req)
		httpbase.ServerError(ctx, fmt.Errorf("failed to update discussion: %w", err))
		return
	}
	httpbase.OK(ctx, nil)

}

// DeleteDiscussion godoc
// @Security     ApiKey
// @Summary      Delete a discussion
// @Description  delete a discussion
// @Tags         Discussion
// @Accept       json
// @Produce      json
// @Param        id path string true "the discussion id"
// @Param        current_user query string true "current user, the owner of the discussion"
// @Success      200  {object}  types.Response "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /discussions/{id} [delete]
func (h *DiscussionHandler) DeleteDiscussion(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = h.c.DeleteDiscussion(ctx, currentUser, idInt)
	if err != nil {
		slog.Error("Failed to delete discussion", "error", err, "id", id)
		httpbase.ServerError(ctx, fmt.Errorf("failed to delete discussion: %w", err))
		return
	}
	httpbase.OK(ctx, nil)
}

// ShowDiscussion godoc
// @Security     ApiKey
// @Summary      Show a discussion and its comments
// @Description  show a discussion
// @Tags         Discussion
// @Accept       json
// @Produce      json
// @Param        id path string true "the discussion id"
// @Success      200  {object}  types.Response{data=component.ShowDiscussionResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /discussions/{id} [get]
func (h *DiscussionHandler) ShowDiscussion(ctx *gin.Context) {
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	d, err := h.c.GetDiscussion(ctx, idInt)
	if err != nil {
		slog.Error("Failed to get discussion", "error", err, "id", id)
		httpbase.ServerError(ctx, fmt.Errorf("failed to get discussion: %w", err))
		return
	}
	httpbase.OK(ctx, d)
}

// ListRepoDiscussions godoc
// @Security     ApiKey
// @Summary      List repo discussions
// @Description  list repo discussions
// @Tags         Discussion
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user"
// @Param        repo_type path string true "repository type" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "namespace"
// @Param        name query string true "name"
// @Success      200  {object}  types.Response{data=component.ListRepoDiscussionResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/discussions [get]
func (h *DiscussionHandler) ListRepoDiscussions(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	repoType := h.getRepoType(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("failed to get namespace and name from request context: %w", err).Error())
		return
	}

	var req component.ListRepoDiscussionRequest
	req.CurrentUser = currentUser
	req.RepoType = types.RepositoryType(repoType)
	req.Namespace = namespace
	req.Name = name
	resp, err := h.c.ListRepoDiscussions(ctx, req)
	if err != nil {
		slog.Error("Failed to list repo discussions", "error", err, "request", req)
		httpbase.ServerError(ctx, fmt.Errorf("failed to list repo discussions: %w", err))
		return
	}
	httpbase.OK(ctx, resp)
}

// CreateDiscussionComment godoc
// @Security     ApiKey
// @Summary      Create a new discussion comment
// @Description  create a new discussion comment
// @Tags         Discussion
// @Accept       json
// @Produce      json
// @Param        id path string true "the discussion id"
// @Param        body body component.CreateCommentRequest true "body"
// @Success      200  {object}  types.Response{data=component.CreateCommentResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /discussions/{id}/comments [post]
func (h *DiscussionHandler) CreateDiscussionComment(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}

	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("invalid discussion id: %w", err).Error())
		return
	}
	var req component.CreateCommentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.CommentableID = idInt
	req.CurrentUser = currentUser

	resp, err := h.c.CreateDiscussionComment(ctx, req)
	if err != nil {
		slog.Error("Failed to create discussion comment", "error", err, "request", req)
		httpbase.ServerError(ctx, fmt.Errorf("failed to create discussion comment: %w", err))
		return
	}
	httpbase.OK(ctx, resp)
}

// UpdateComment godoc
// @Security     ApiKey
// @Summary      Update a comment content by id
// @Description  update a comment content by id
// @Tags         Discussion
// @Accept       json
// @Produce      json
// @Param        id path string true "the comment id"
// @Param        current_user query string true "current user, the owner of the comment"
// @Param        body body component.UpdateCommentRequest true "body"
// @Success      200  {object}  types.Response "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /comments/{id} [put]
func (h *DiscussionHandler) UpdateComment(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("invalid comment id: %w", err).Error())
		return
	}

	var req component.UpdateCommentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = h.c.UpdateComment(ctx, currentUser, idInt, req.Content)
	if err != nil {
		slog.Error("Failed to update comment", "error", err, "request", req)
		httpbase.ServerError(ctx, fmt.Errorf("failed to update comment: %w", err))
		return
	}
	httpbase.OK(ctx, nil)
}

// DeleteDiscussionComment godoc
// @Security     ApiKey
// @Summary      Delete a comment by id
// @Description  delete a comment by id
// @Tags         Discussion
// @Accept       json
// @Produce      json
// @Param        id path string true "the comment id"
// @Param        current_user query string true "current user, the owner of the comment"
// @Success      200  {object}  types.Response "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /comments/{id} [delete]
func (h *DiscussionHandler) DeleteComment(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("invalid comment id: %w", err).Error())
		return
	}
	err = h.c.DeleteComment(ctx, currentUser, idInt)
	if err != nil {
		slog.Error("Failed to delete comment", "error", err, "id", id)
		httpbase.ServerError(ctx, fmt.Errorf("failed to delete comment: %w", err))
		return
	}
	httpbase.OK(ctx, nil)
}

// ListDiscussionComments godoc
// @Security     ApiKey
// @Summary      List discussion comments
// @Description  list discussion comments
// @Tags         Discussion
// @Accept       json
// @Produce      json
// @Param        id path string true "the discussion id"
// @Success      200  {object}  types.Response{data=[]component.DiscussionResponse_Comment} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /discussions/{id}/comments [get]
func (h *DiscussionHandler) ListDiscussionComments(ctx *gin.Context) {
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("invalid discussion id: %w", err).Error())
	}
	comments, err := h.c.ListDiscussionComments(ctx, idInt)
	if err != nil {
		slog.Error("Failed to list discussion comments", "error", err, "id", id)
		httpbase.ServerError(ctx, fmt.Errorf("failed to list discussion comments: %w", err))
		return
	}
	httpbase.OK(ctx, comments)
}

func (h *DiscussionHandler) getRepoType(ctx *gin.Context) types.RepositoryType {
	repoType := ctx.Param("repo_type")
	repoType = strings.TrimRight(repoType, "s")
	return types.RepositoryType(repoType)
}
