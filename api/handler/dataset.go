package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

var Sorts = []string{"trending", "recently_update", "most_download", "most_favorite"}
var Sources = []string{"opencsg", "huggingface", "local"}

func NewDatasetHandler(config *config.Config) (*DatasetHandler, error) {
	tc, err := component.NewDatasetComponent(config)
	if err != nil {
		return nil, err
	}
	return &DatasetHandler{
		c:  tc,
		sc: component.NewSensitiveComponent(config),
	}, nil
}

type DatasetHandler struct {
	c  *component.DatasetComponent
	sc component.SensitiveChecker
}

// CreateDataset   godoc
// @Security     ApiKey
// @Summary      Create a new dataset
// @Description  create a new dataset
// @Tags         Dataset
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user, the owner"
// @Param        body body types.CreateDatasetReq true "body"
// @Success      200  {object}  types.Response{data=types.Dataset} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /datasets [post]
func (h *DatasetHandler) Create(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req *types.CreateDatasetReq
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

	dataset, err := h.c.Create(ctx, req)
	if err != nil {
		slog.Error("Failed to create dataset", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Create dataset succeed", slog.String("dataset", dataset.Name))
	respData := gin.H{
		"data": dataset,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetVisiableDatasets godoc
// @Security     ApiKey
// @Summary      Get Visiable datasets for current user
// @Description  get visiable datasets for current user
// @Tags         Dataset
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
// @Router       /datasets [get]
func (h *DatasetHandler) Index(ctx *gin.Context) {
	filter := new(types.RepoFilter)
	filter.Tags = parseTagReqs(ctx)
	filter.Username = httpbase.GetCurrentUser(ctx)
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
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

	datasets, total, err := h.c.Index(ctx, filter, per, page)
	if err != nil {
		slog.Error("Failed to get datasets", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get public datasets succeed", slog.Int("count", total))
	respData := gin.H{
		"data":  datasets,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// UpdateDataset   godoc
// @Security     ApiKey
// @Summary      Update a exists dataset
// @Description  update a exists dataset
// @Tags         Dataset
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user, the owner"
// @Param        body body types.UpdateDatasetReq true "body"
// @Success      200  {object}  types.Response{data=database.Dataset} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /datasets/{namespace}/{name} [put]
func (h *DatasetHandler) Update(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req *types.UpdateDatasetReq
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

	dataset, err := h.c.Update(ctx, req)
	if err != nil {
		slog.Error("Failed to update dataset", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update dataset succeed", slog.String("dataset", dataset.Name))
	httpbase.OK(ctx, dataset)
}

// DeleteDataset   godoc
// @Security     ApiKey
// @Summary      Delete a exists dataset
// @Description  delete a exists dataset
// @Tags         Dataset
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user, the owner"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /datasets/{namespace}/{name} [delete]
func (h *DatasetHandler) Delete(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = h.c.Delete(ctx, namespace, name, currentUser)
	if err != nil {
		slog.Error("Failed to delete dataset", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Delete dataset succeed", slog.String("dataset", name))
	httpbase.OK(ctx, nil)
}

// GetDataset      godoc
// @Security     ApiKey
// @Summary      Get dataset detail
// @Description  get dataset detail
// @Tags         Dataset
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{data=types.Dataset} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /datasets/{namespace}/{name} [get]
func (h *DatasetHandler) Show(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	detail, err := h.c.Show(ctx, namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		slog.Error("Failed to get dataset", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get dataset succeed", slog.String("dataset", name))
	httpbase.OK(ctx, detail)
}

// DatasetRelations      godoc
// @Security     ApiKey
// @Summary      Get dataset related assets
// @Tags         Dataset
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current_user"
// @Success      200  {object}  types.Response{data=types.Relations} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /datasets/{namespace}/{name}/relations [get]
func (h *DatasetHandler) Relations(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	detail, err := h.c.Relations(ctx, namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		slog.Error("Failed to get dataset relations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, detail)
}

func getFilterFromContext(ctx *gin.Context, filter *types.RepoFilter) *types.RepoFilter {
	filter.Search = ctx.Query("search")
	filter.Sort = ctx.Query("sort")
	if filter.Sort == "" {
		filter.Sort = "recently_update"
	}
	filter.Source = ctx.Query("source")
	return filter
}

// DatasetFiles      godoc
// @Security     ApiKey
// @Summary      Get all files of a dataset
// @Tags         Dataset
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{data=types.File} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /datasets/{namespace}/{name}/all_files [get]
func (h *DatasetHandler) AllFiles(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req types.GetAllFilesReq
	req.Namespace = namespace
	req.Name = name
	req.RepoType = types.DatasetRepo
	req.CurrentUser = httpbase.GetCurrentUser(ctx)
	detail, err := h.c.AllFiles(ctx, req)
	if err != nil {
		if errors.Is(err, component.ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		slog.Error("Failed to get dataset all files", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, detail)
}
