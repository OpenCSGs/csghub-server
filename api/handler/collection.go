package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewCollectionHandler(cfg *config.Config) (*CollectionHandler, error) {
	cc, err := component.NewCollectionComponent(cfg)
	if err != nil {
		return nil, err
	}
	sc, err := component.NewSensitiveComponent(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	return &CollectionHandler{
		collection: cc,
		sensitive:  sc,
	}, nil
}

type CollectionHandler struct {
	collection component.CollectionComponent
	sensitive  component.SensitiveComponent
}

// GetCollections godoc
// @Summary      get all collections
// @Description  get all collections
// @Tags         Collection
// @Param        search query string false "search text"
// @Param        sort query string false "sort by" default("trending")
// @Param        per query int false "per" default(50)
// @Param        page query int false "per page" default(1)
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Collection,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /collections [get]
func (c *CollectionHandler) Index(ctx *gin.Context) {
	filter := new(types.CollectionFilter)
	filter = getCollectionFilter(ctx, filter)
	if !slices.Contains(types.CollectionSorts, filter.Sort) {
		msg := fmt.Sprintf("sort parameter must be one of %v", types.CollectionSorts)
		err := errorx.ReqParamInvalid(errors.New(msg),
			errorx.Ctx().
				Set("param", "sort").
				Set("provided", filter.Sort).
				Set("allowed", types.CollectionSorts))
		slog.ErrorContext(ctx.Request.Context(), "Bad request format,", slog.String("error", msg))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	collections, total, err := c.collection.GetCollections(ctx.Request.Context(), filter, per, page)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to load collections", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  collections,
		"total": total,
	}

	ctx.JSON(http.StatusOK, respData)
}

// CreateCollection godoc
// @Security     JWT token
// @Summary      create a collection
// @Description  create a collection
// @Tags         Collection
// @Accept       json
// @Produce      json
// @Param        body body types.CreateCollectionReq true "body"
// @Success      200  {object}  types.Response{data=types.Collection} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /collections [post]
func (c *CollectionHandler) Create(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req *types.CreateCollectionReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	_, err := c.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	req.Username = currentUser
	collection, err := c.collection.CreateCollection(ctx.Request.Context(), *req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create collection", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, collection)
}

// GetCollection godoc
// @Summary      get a collection detail
// @Description  get a collection detail
// @Tags         Collection
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  types.Response{data=types.Collection} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /collections/{id} [get]
func (c *CollectionHandler) GetCollection(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	collection, err := c.collection.GetCollection(ctx.Request.Context(), currentUser, id)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get collection", slog.Any("error", err))
		if errors.Is(err, errorx.ErrDatabaseNoRows) {
			httpbase.NotFoundError(ctx, err)
			return
		} else {
			httpbase.ServerError(ctx, err)
			return
		}
	}

	httpbase.OK(ctx, collection)
}

// UpdateCollection godoc
// @Security     JWT token
// @Summary      update a collection
// @Description  update a collection
// @Tags         Collection
// @Accept       json
// @Produce      json
// @Param        body body types.CreateCollectionReq true "body"
// @Param        id path string true "id"
// @Success      200  {object}  types.Response{data=types.Collection} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /collections/{id} [put]
func (c *CollectionHandler) UpdateCollection(ctx *gin.Context) {
	var req *types.CreateCollectionReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	_, err := c.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.ID = id

	collection, err := c.collection.UpdateCollection(ctx.Request.Context(), *req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create space", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, collection)
}

// DeleteCollection godoc
// @Security     JWT token
// @Summary      Delete a exists collection
// @Description  delete a exists collection
// @Tags         Collection
// @Accept       json
// @Produce      json
// @Param        id path string true "id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /collections/{id} [delete]
func (c *CollectionHandler) DeleteCollection(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = c.collection.DeleteCollection(ctx.Request.Context(), id, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete collection", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// AddRepoToCollection godoc
// @Security     JWT token
// @Summary      Add repos to a collection
// @Description  Add repos to a collection
// @Tags         Collection
// @Accept       json
// @Produce      json
// @Param        body body types.UpdateCollectionReposReq true "body"
// @Param        id path string true "id"
// @Success      200  {object}  types.Response{data=database.Collection} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /collections/{id}/repos [post]
func (c *CollectionHandler) AddRepoToCollection(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req *types.UpdateCollectionReposReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Username = currentUser
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.ID = id

	if len(req.Remarks) > 0 {
		for repoId, remark := range req.Remarks {
			_, err = c.sensitive.CheckRequestV2(ctx.Request.Context(), &types.UpdateCollectionRepoReq{
				Remark: remark,
			})
			if err != nil {
				slog.ErrorContext(ctx.Request.Context(), "Failed to check sensitive request",
					slog.Int64("collection_id", req.ID),
					slog.Int64("repo_id", repoId),
					slog.Any("error", err))
				httpbase.ServerError(ctx, fmt.Errorf("sensitive check failed: %w", err))
				return
			}
		}
	}

	err = c.collection.AddReposToCollection(ctx.Request.Context(), *req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create collection", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// RemoveRepoFromCollection godoc
// @Security     JWT token
// @Summary      remove repos from a collection
// @Description  remove repos from a collection
// @Tags         Collection
// @Accept       json
// @Produce      json
// @Param        body body types.UpdateCollectionReposReq true "body"
// @Param        id path string true "id"
// @Success      200  {object}  types.Response{data=types.Collection} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /collections/{id}/repos [delete]
func (c *CollectionHandler) RemoveRepoFromCollection(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req *types.UpdateCollectionReposReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Username = currentUser
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.ID = id

	err = c.collection.RemoveReposFromCollection(ctx.Request.Context(), *req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create collection", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// UpdateCollectionRepo godoc
// @Security     JWT token
// @Summary      update repo remark
// @Description  update repo remark
// @Tags         Collection
// @Accept       json
// @Produce      json
// @Param        body body types.UpdateCollectionRepoReq true "body"
// @Param        id path string true "id"
// @Param        repo_id path string true "repo_id"
// @Success      200  {object}  types.Response{data=nil} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /collections/{id}/repos/{repo_id} [put]
func (c *CollectionHandler) UpdateCollectionRepo(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req *types.UpdateCollectionRepoReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}

	collectionId, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	req.ID = collectionId
	repoId, err := strconv.ParseInt(ctx.Param("repo_id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}
	req.RepoID = repoId

	_, err = c.sensitive.CheckRequestV2(ctx.Request.Context(), &types.UpdateCollectionRepoReq{
		Remark: req.Remark,
	})
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to check sensitive request",
			slog.Int64("collection_id", collectionId),
			slog.Int64("repo_id", repoId),
			slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("sensitive check failed: %w", err))
		return
	}

	req.Username = currentUser
	err = c.collection.UpdateCollectionRepo(ctx.Request.Context(), *req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to update repo remark", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

func getCollectionFilter(ctx *gin.Context, filter *types.CollectionFilter) *types.CollectionFilter {
	filter.Search = ctx.Query("search")
	filter.Sort = ctx.Query("sort")
	if filter.Sort == "" {
		filter.Sort = "trending"
	}
	return filter
}
