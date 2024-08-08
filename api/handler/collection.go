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
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewCollectionHandler(cfg *config.Config) (*CollectionHandler, error) {
	cc, err := component.NewCollectionComponent(cfg)
	if err != nil {
		return nil, err
	}
	return &CollectionHandler{
		cc: cc,
		sc: component.NewSensitiveComponent(cfg),
	}, nil
}

type CollectionHandler struct {
	cc *component.CollectionComponent
	sc component.SensitiveChecker
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
		slog.Error("Bad request format,", slog.String("error", msg))
		ctx.JSON(http.StatusBadRequest, gin.H{"message": msg})
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	collections, total, err := c.cc.GetCollections(ctx, filter, per, page)
	if err != nil {
		slog.Error("Failed to load collections", "error", err)
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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req *types.CreateCollectionReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	_, err := c.sc.CheckRequest(ctx, req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	req.Username = currentUser
	collection, err := c.cc.CreateCollection(ctx, *req)
	if err != nil {
		slog.Error("Failed to create collection", slog.Any("error", err))
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
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	collection, err := c.cc.GetCollection(ctx, currentUser, id)
	if err != nil {
		slog.Error("Failed to create space", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
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
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req *types.CreateCollectionReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	_, err := c.sc.CheckRequest(ctx, req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.ID = id

	collection, err := c.cc.UpdateCollection(ctx, *req)
	if err != nil {
		slog.Error("Failed to create space", slog.Any("error", err))
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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = c.cc.DeleteCollection(ctx, id, currentUser)
	if err != nil {
		slog.Error("Failed to delete collection", slog.Any("error", err))
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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req *types.UpdateCollectionReposReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Username = currentUser
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.ID = id

	err = c.cc.AddReposToCollection(ctx, *req)
	if err != nil {
		slog.Error("Failed to create collection", slog.Any("error", err))
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
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req *types.UpdateCollectionReposReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Username = currentUser
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.ID = id

	err = c.cc.RemoveReposFromCollection(ctx, *req)
	if err != nil {
		slog.Error("Failed to create collection", slog.Any("error", err))
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
