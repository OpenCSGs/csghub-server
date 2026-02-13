package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type DiscussionHandler struct {
	discussion component.DiscussionComponent
	sensitive  component.SensitiveComponent
}

func NewDiscussionHandler(cfg *config.Config) (*DiscussionHandler, error) {
	c, err := component.NewDiscussionComponent(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create discussion component: %w", err)
	}
	sc, err := component.NewSensitiveComponent(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create sensitive component: %w", err)
	}
	return &DiscussionHandler{
		discussion: c,
		sensitive:  sc,
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
// @Param        body body types.CreateRepoDiscussionRequest true "body"
// @Success      200  {object}  types.Response{data=types.CreateDiscussionResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/discussions [post]
func (h *DiscussionHandler) CreateRepoDiscussion(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	repoType := h.getRepoType(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var req types.CreateRepoDiscussionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	_, err = h.sensitive.CheckRequestV2(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, errorx.ErrSensitiveInfoNotAllowed)
		return
	}

	req.CurrentUser = currentUser
	req.RepoType = types.RepositoryType(repoType)
	req.Namespace = namespace
	req.Name = name
	resp, err := h.discussion.CreateRepoDiscussion(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to create repo discussion", "error", err, "request", req)
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
// @Param        body body types.UpdateDiscussionRequest true "body"
// @Success      200  {object}  types.Response "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /discussions/{id} [put]
func (h *DiscussionHandler) UpdateDiscussion(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, "invalid discussion id:"+id)
		return
	}
	var req types.UpdateDiscussionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	_, err = h.sensitive.CheckRequestV2(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, errorx.ErrSensitiveInfoNotAllowed)
		return
	}

	req.ID = idInt
	req.CurrentUser = currentUser
	err = h.discussion.UpdateDiscussion(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to update discussion", "error", err, "request", req)
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
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = h.discussion.DeleteDiscussion(ctx.Request.Context(), currentUser, idInt)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete discussion", "error", err, "id", id)
		httpbase.ServerError(ctx, fmt.Errorf("failed to delete discussion: %w", err))
		return
	}
	httpbase.OK(ctx, nil)
}

// ShowDiscussion godoc
// @Security     ApiKey
// @Summary      Show a discussion and its comments with pagination
// @Description  show a discussion
// @Tags         Discussion
// @Accept       json
// @Produce      json
// @Param        id path string true "the discussion id"
// @Param        per query int false "per" default(10)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.Response{data=types.ShowDiscussionResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /discussions/{id} [get]
func (h *DiscussionHandler) ShowDiscussion(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx) // optional, can be empty if not logged in
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	cPer, cPage, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	d, err := h.discussion.GetDiscussion(ctx.Request.Context(), currentUser, idInt, cPer, cPage)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
		} else if errors.Is(err, errorx.ErrDatabaseNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			slog.ErrorContext(ctx.Request.Context(), "Failed to get discussion", "error", err, "id", id)
			httpbase.ServerError(ctx, fmt.Errorf("failed to get discussion: %w", err))
		}
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
// @Param        per query int false "per" default(10)
// @Param        page query int false "per page" default(1)
// @Param        current_user query string false "current user"
// @Param        repo_type path string true "repository type" Enums(models,datasets,codes,spaces)
// @Param        namespace path string true "namespace"
// @Param        name query string true "name"
// @Success      200  {object}  types.Response{data=types.ListRepoDiscussionResponse} "OK"
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
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var req types.ListRepoDiscussionRequest
	req.CurrentUser = currentUser
	req.RepoType = types.RepositoryType(repoType)
	req.Namespace = namespace
	req.Name = name

	resp, total, err := h.discussion.ListRepoDiscussions(ctx.Request.Context(), req, per, page)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
		} else if errors.Is(err, errorx.ErrDatabaseNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			slog.ErrorContext(ctx.Request.Context(), "Failed to list repo discussions", "error", err, "request", req)
			httpbase.ServerError(ctx, fmt.Errorf("failed to list repo discussions: %w", err))
		}
		return
	}
	httpbase.OKWithTotal(ctx, resp, total)
}

// CreateDiscussionComment godoc
// @Security     ApiKey
// @Summary      Create a new discussion comment
// @Description  create a new discussion comment
// @Tags         Discussion
// @Accept       json
// @Produce      json
// @Param        id path string true "the discussion id"
// @Param        body body types.CreateCommentRequest true "body"
// @Success      200  {object}  types.Response{data=types.CreateCommentResponse} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /discussions/{id}/comments [post]
func (h *DiscussionHandler) CreateDiscussionComment(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("invalid discussion id: %w", err).Error())
		return
	}
	var req types.CreateCommentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	_, err = h.sensitive.CheckRequestV2(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, errorx.ErrSensitiveInfoNotAllowed)
		return
	}

	req.CommentableID = idInt
	req.CurrentUser = currentUser

	resp, err := h.discussion.CreateDiscussionComment(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create discussion comment", "error", err, "request", req)
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
// @Param        body body types.UpdateCommentRequest true "body"
// @Success      200  {object}  types.Response "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /comments/{id} [put]
func (h *DiscussionHandler) UpdateComment(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("invalid comment id: %w", err).Error())
		return
	}

	var req types.UpdateCommentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	_, err = h.sensitive.CheckRequestV2(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, errorx.ErrSensitiveInfoNotAllowed)
		return
	}

	err = h.discussion.UpdateComment(ctx.Request.Context(), currentUser, idInt, req.Content)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to update comment", "error", err, "request", req)
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
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("invalid comment id: %w", err).Error())
		return
	}
	err = h.discussion.DeleteComment(ctx.Request.Context(), currentUser, idInt)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete comment", "error", err, "id", id)
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
// @Param        per query int false "per" default(10)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.Response{data=[]types.DiscussionResponse_Comment} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /discussions/{id}/comments [get]
func (h *DiscussionHandler) ListDiscussionComments(ctx *gin.Context) {
	// dont check user login state, allow anonymous access
	currentUser := httpbase.GetCurrentUser(ctx)
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("invalid discussion id: %w", err).Error())
	}

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	comments, total, err := h.discussion.ListDiscussionComments(ctx.Request.Context(), currentUser, idInt, per, page)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
		} else if errors.Is(err, errorx.ErrDatabaseNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			slog.ErrorContext(ctx.Request.Context(), "Failed to list discussion comments", "error", err, "id", id)
			httpbase.ServerError(ctx, fmt.Errorf("failed to list discussion comments: %w", err))
		}
		return
	}
	httpbase.OKWithTotal(ctx, comments, total)
}

func (h *DiscussionHandler) getRepoType(ctx *gin.Context) types.RepositoryType {
	repoType := ctx.Param("repo_type")
	repoType = strings.TrimRight(repoType, "s")
	return types.RepositoryType(repoType)
}
