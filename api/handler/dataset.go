package handler

import (
	"errors"
	"fmt"
	"log/slog"
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

func NewDatasetHandler(config *config.Config) (*DatasetHandler, error) {
	tc, err := component.NewDatasetComponent(config)
	if err != nil {
		return nil, err
	}
	sc, err := component.NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	repo, err := component.NewRepoComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating repo component:%w", err)
	}
	return &DatasetHandler{
		dataset:   tc,
		sensitive: sc,
		repo:      repo,
		config:    config,
	}, nil
}

type DatasetHandler struct {
	dataset   component.DatasetComponent
	sensitive component.SensitiveComponent
	repo      component.RepoComponent
	config    *config.Config
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
	var req *types.CreateDatasetReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}

	if req.Namespace == "" {
		req.Namespace = currentUser
	}
	req.Username = currentUser
	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, errorx.ErrSensitiveInfoNotAllowed)
		return
	}
	if !req.Private && !h.allowCreatePublic() {
		httpbase.BadRequestWithExt(ctx, errorx.ErrForbiddenMsg("creating public dataset is not allowed"))
		return
	}

	dataset, err := h.dataset.Create(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		} else if errors.Is(err, errorx.ErrDatabaseDuplicateKey) {
			httpbase.BadRequestWithExt(ctx, err)
		} else {
			slog.ErrorContext(ctx.Request.Context(), "Failed to create dataset", slog.Any("error", err))
			httpbase.ServerError(ctx, err)
		}
		return
	}
	slog.Info("Create dataset succeed", slog.String("dataset", dataset.Name))
	httpbase.OK(ctx, dataset)
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
// @Param        xnet_migration_status query string false "filter by xnet migration status" Enums(pending, running, completed, failed)
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
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	filter = getFilterFromContext(ctx, filter)
	if !slices.Contains(types.Sorts, filter.Sort) {
		err = fmt.Errorf("sort parameter must be one of %v", types.Sorts)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("query", "sort_filter"))
		slog.ErrorContext(ctx.Request.Context(), "Bad sort request format,", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	if filter.Source != "" && !slices.Contains(types.Sources, filter.Source) {
		err = fmt.Errorf("source parameter must be one of %v", types.Sources)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("query", "source_filter"))
		slog.ErrorContext(ctx.Request.Context(), "Bad source request format,", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	qNeedOpWeight := ctx.Query("need_op_weight")
	needOpWeight, err := strconv.ParseBool(qNeedOpWeight)
	if err != nil {
		needOpWeight = false
	}
	datasets, total, err := h.dataset.Index(ctx.Request.Context(), filter, per, page, needOpWeight)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get datasets", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get public datasets succeed", slog.Int("count", total))
	httpbase.OKWithTotal(ctx, datasets, total)
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
	var req *types.UpdateDatasetReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, errorx.ReqBodyFormat(err, nil))
		return
	}

	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, errorx.ErrSensitiveInfoNotAllowed)
		return
	}
	req.Username = currentUser

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	req.Namespace = namespace
	req.Name = name

	dataset, err := h.dataset.Update(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update dataset", slog.Any("error", err))
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
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
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	err = h.dataset.Delete(ctx.Request.Context(), namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete dataset", slog.Any("error", err))
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
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        need_multi_sync query bool false "need multi sync" default(false)
// @Success      200  {object}  types.Response{data=types.Dataset} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /datasets/{namespace}/{name} [get]
func (h *DatasetHandler) Show(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)

	qNeedOpWeight := ctx.Query("need_op_weight")
	needOpWeight, err := strconv.ParseBool(qNeedOpWeight)
	if err != nil {
		needOpWeight = false
	}
	qNeedMultiSync := ctx.Query("need_multi_sync")
	needMultiSync, err := strconv.ParseBool(qNeedMultiSync)
	if err != nil {
		needMultiSync = false
	}
	detail, err := h.dataset.Show(ctx.Request.Context(), namespace, name, currentUser, needOpWeight, needMultiSync)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get dataset", slog.Any("error", err))
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
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	detail, err := h.dataset.Relations(ctx.Request.Context(), namespace, name, currentUser)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to get dataset relations", slog.Any("error", err))
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
	filter.Status = ctx.Query("status")

	xnetMigrationStatus := ctx.Query("xnet_migration_status")
	if xnetMigrationStatus != "" {
		status := types.XnetMigrationTaskStatus(xnetMigrationStatus)
		if status == types.XnetMigrationTaskStatusPending ||
			status == types.XnetMigrationTaskStatusRunning ||
			status == types.XnetMigrationTaskStatusCompleted ||
			status == types.XnetMigrationTaskStatusFailed {
			filter.XnetMigrationStatus = &status
		}
	}

	return filter
}

func (h *DatasetHandler) allowCreatePublic() bool {
	return h.config.Dataset.AllowCreatePublicDataset
}
