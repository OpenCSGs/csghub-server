package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewTagHandler(config *config.Config) (*TagsHandler, error) {
	tc, err := component.NewTagComponent(config)
	if err != nil {
		return nil, err
	}
	return &TagsHandler{
		tc: tc,
	}, nil
}

type TagsHandler struct {
	tc component.TagComponent
}

// GetAllTags godoc
// @Security     ApiKey
// @Summary      Get all tags
// @Description  Get all tags
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Param		 category query string false "category name"
// @Param		 scope query string false "scope name" Enums(model, dataset)
// @Success      200  {object}  types.ResponseWithTotal{data=[]database.Tag} "tags"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags [get]
func (t *TagsHandler) AllTags(ctx *gin.Context) {
	//TODO:validate inputs
	category := ctx.Query("category")
	scope := ctx.Query("scope")
	tags, err := t.tc.AllTagsByScopeAndCategory(ctx, scope, category)
	if err != nil {
		slog.Error("Failed to load tags", slog.Any("category", category), slog.Any("scope", scope), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data": tags,
	}
	ctx.JSON(http.StatusOK, respData)
}

// CreateTag     godoc
// @Security     ApiKey
// @Summary      Create new tag
// @Description  Create new tag
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Param        body body types.CreateTag true "body"
// @Success      200  {object}  types.Response{database.Tag} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags [post]
func (t *TagsHandler) CreateTag(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	if userName == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var req types.CreateTag
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	tag, err := t.tc.CreateTag(ctx, userName, req)
	if err != nil {
		slog.Error("Failed to create tag", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": tag})
}

// GetTag        godoc
// @Security     ApiKey
// @Summary      Get a tag by id
// @Description  Get a tag by id
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Param		 id path  string  true  "id of the tag"
// @Success      200  {object}  types.Response{database.Tag} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags/{id} [get]
func (t *TagsHandler) GetTagByID(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	if userName == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	tag, err := t.tc.GetTagByID(ctx, userName, id)
	if err != nil {
		slog.Error("Failed to get tag", slog.Int64("id", id), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": tag})
}

// UpdateTag     godoc
// @Security     ApiKey
// @Summary      Update a tag by id
// @Description  Update a tag by id
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Param		 id path  string  true  "id of the tag"
// @Param        body body types.UpdateTag true "body"
// @Success      200  {object}  types.Response{database.Tag} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags/{id} [put]
func (t *TagsHandler) UpdateTag(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	if userName == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req types.UpdateTag
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	tag, err := t.tc.UpdateTag(ctx, userName, id, req)
	if err != nil {
		slog.Error("Failed to update tag", slog.Int64("id", id), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": tag})
}

// DeleteTag     godoc
// @Security     ApiKey
// @Summary      Delete a tag by id
// @Description  Delete a tag by id
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Param		 id path  string  true  "id of the tag"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags/{id} [delete]
func (t *TagsHandler) DeleteTag(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	if userName == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = t.tc.DeleteTag(ctx, userName, id)
	if err != nil {
		slog.Error("Failed to delete tag", slog.Int64("id", id), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, nil)
}
