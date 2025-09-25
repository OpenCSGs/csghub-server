package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewTagHandler(config *config.Config) (*TagsHandler, error) {
	tc, err := component.NewTagComponent(config)
	if err != nil {
		return nil, err
	}
	return &TagsHandler{
		tag: tc,
	}, nil
}

type TagsHandler struct {
	tag component.TagComponent
}

// GetAllTags godoc
// @Security     ApiKey
// @Summary      Get all tags
// @Description  Get all tags
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Param		 category query string false "category name"
// @Param		 scope query string false "scope name" Enums(model, dataset, code, space, prompt)
// @Param		 built_in query bool false "built_in"
// @Param		 search query string false "search on name and show_name fields"
// @Success      200  {object}  types.Response{data=[]types.RepoTag} "tags"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags [get]
func (t *TagsHandler) AllTags(ctx *gin.Context) {
	filter := new(types.TagFilter)
	err := ctx.ShouldBindQuery(filter)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	tags, err := t.tag.AllTags(ctx.Request.Context(), filter)
	if err != nil {
		slog.Error("Failed to load tags", slog.Any("filter", filter), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, tags)
}

// CreateTag     godoc
// @Security     ApiKey
// @Summary      Create new tag
// @Description  Create new tag, used for admin
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Param        body body types.CreateTag true "body"
// @Success      200  {object}  types.Response{data=database.Tag} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags [post]
func (t *TagsHandler) CreateTag(ctx *gin.Context) {
	var req types.CreateTag
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	tag, err := t.tag.CreateTag(ctx.Request.Context(), req)
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
// @Description  Get a tag by id, used for admin
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Param		 id path  string  true  "id of the tag"
// @Success      200  {object}  types.Response{data=database.Tag} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags/{id} [get]
func (t *TagsHandler) GetTagByID(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	tag, err := t.tag.GetTagByID(ctx.Request.Context(), id)
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
// @Description  Update a tag by id, used for admin
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Param		 id path  string  true  "id of the tag"
// @Param        body body types.UpdateTag true "body"
// @Success      200  {object}  types.Response{data=database.Tag} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags/{id} [put]
func (t *TagsHandler) UpdateTag(ctx *gin.Context) {
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
	tag, err := t.tag.UpdateTag(ctx.Request.Context(), id, req)
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
// @Description  Delete a tag by id, used for admin
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Param		 id path  string  true  "id of the tag"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags/{id} [delete]
func (t *TagsHandler) DeleteTag(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = t.tag.DeleteTag(ctx.Request.Context(), id)
	if err != nil {
		slog.Error("Failed to delete tag", slog.Int64("id", id), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, nil)
}

// GetAllCategories godoc
// @Security     ApiKey
// @Summary      Get all Categories
// @Description  Get all Categories
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{data=[]types.RepoTagCategory} "categores"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags/categories [get]
func (t *TagsHandler) AllCategories(ctx *gin.Context) {
	categories, err := t.tag.AllCategories(ctx.Request.Context())
	if err != nil {
		slog.Error("Failed to load categories", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, categories)
}

// CreateCategory     godoc
// @Security     ApiKey
// @Summary      Create new category
// @Description  Create new category, used for admin
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Param        body body types.CreateCategory true "body"
// @Success      200  {object}  types.Response{data=database.TagCategory} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags/categories [post]
func (t *TagsHandler) CreateCategory(ctx *gin.Context) {
	var req types.CreateCategory
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	category, err := t.tag.CreateCategory(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to create category", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": category})
}

// UpdateCategory     godoc
// @Security     ApiKey
// @Summary      Create new category
// @Description  Create new category, used for admin
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Param        body body types.UpdateCategory true "body"
// @Success      200  {object}  types.Response{data=database.TagCategory} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags/categories/id [put]
func (t *TagsHandler) UpdateCategory(ctx *gin.Context) {
	var req types.UpdateCategory
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	category, err := t.tag.UpdateCategory(ctx.Request.Context(), req, id)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to update category", slog.Any("req", req), slog.Any("id", id), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": category})
}

// DeleteCategory  godoc
// @Security     ApiKey
// @Summary      Delete a category by id
// @Description  Delete a category by id, used for admin
// @Tags         Tag
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /tags/categories/id [delete]
func (t *TagsHandler) DeleteCategory(ctx *gin.Context) {

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = t.tag.DeleteCategory(ctx.Request.Context(), id)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("Failed to delete category", slog.Any("id", id), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, nil)
}
