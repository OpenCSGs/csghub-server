package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewOrganizationHandler(config *config.Config) (*OrganizationHandler, error) {
	sc, err := component.NewSpaceComponent(config)
	if err != nil {
		return nil, err
	}
	cc, err := component.NewCodeComponent(config)
	if err != nil {
		return nil, err
	}
	mc, err := component.NewModelComponent(config)
	if err != nil {
		return nil, err
	}
	dsc, err := component.NewDatasetComponent(config)
	if err != nil {
		return nil, err
	}
	colc, err := component.NewCollectionComponent(config)
	if err != nil {
		return nil, err
	}
	return &OrganizationHandler{
		sc:   sc,
		cc:   cc,
		mc:   mc,
		dsc:  dsc,
		colc: colc,
	}, nil
}

type OrganizationHandler struct {
	sc   *component.SpaceComponent
	cc   *component.CodeComponent
	mc   *component.ModelComponent
	dsc  *component.DatasetComponent
	colc *component.CollectionComponent
}

// GetOrganizationModels godoc
// @Security     ApiKey
// @Summary      Get organization models
// @Description  get organization models
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        namespace path string true "org name"
// @Param        current_user query string true "current user name"
// @Param        per query int false "page size"
// @Param        page query int false "current page number"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Model,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/models [get]
func (h *OrganizationHandler) Models(ctx *gin.Context) {
	var req types.OrgModelsReq
	req.Namespace = ctx.Param("namespace")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Page = page
	req.PageSize = per
	models, total, err := h.mc.OrgModels(ctx, &req)
	if err != nil {
		slog.Error("Failed to get org models", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get org models succeed", slog.String("org", req.Namespace))

	respData := gin.H{
		"message": "OK",
		"data":    models,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetOrganizationDatasets godoc
// @Security     ApiKey
// @Summary      Get organization datasets
// @Description  get organization datasets
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        namespace path string true "org name"
// @Param        current_user query string true "current user name"
// @Param        per query int false "page size"
// @Param        page query int false "current page number"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Dataset,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/datasets [get]
func (h *OrganizationHandler) Datasets(ctx *gin.Context) {
	var req types.OrgDatasetsReq
	req.Namespace = ctx.Param("namespace")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Page = page
	req.PageSize = per
	datasets, total, err := h.dsc.OrgDatasets(ctx, &req)
	if err != nil {
		slog.Error("Failed to get org datasets", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get org datasets succeed", slog.String("org", req.Namespace))

	respData := gin.H{
		"message": "OK",
		"data":    datasets,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetOrganizationCodes godoc
// @Security     ApiKey
// @Summary      Get organization codes
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        namespace path string true "org name"
// @Param        current_user query string true "current user name"
// @Param        per query int false "page size"
// @Param        page query int false "current page number"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Code,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/codes [get]
func (h *OrganizationHandler) Codes(ctx *gin.Context) {
	var req types.OrgCodesReq
	req.Namespace = ctx.Param("namespace")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Page = page
	req.PageSize = per
	datasets, total, err := h.cc.OrgCodes(ctx, &req)
	if err != nil {
		slog.Error("Failed to get org codes", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get org codes succeed", slog.String("org", req.Namespace))

	respData := gin.H{
		"message": "OK",
		"data":    datasets,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetOrganizationSpaces godoc
// @Security     ApiKey
// @Summary      Get organization Spaces
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        namespace path string true "org name"
// @Param        current_user query string true "current user name"
// @Param        per query int false "page size"
// @Param        page query int false "current page number"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Space,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/spaces [get]
func (h *OrganizationHandler) Spaces(ctx *gin.Context) {
	var req types.OrgSpacesReq
	req.Namespace = ctx.Param("namespace")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Page = page
	req.PageSize = per
	datasets, total, err := h.sc.OrgSpaces(ctx, &req)
	if err != nil {
		slog.Error("Failed to get org spaces", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get org spaces succeed", slog.String("org", req.Namespace))

	respData := gin.H{
		"message": "OK",
		"data":    datasets,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetOrganizationCollections godoc
// @Security     ApiKey
// @Summary      Get organization Collections
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        namespace path string true "org name"
// @Param        current_user query string true "current user name"
// @Param        per query int false "page size"
// @Param        page query int false "current page number"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Collection,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/collections [get]
func (h *OrganizationHandler) Collections(ctx *gin.Context) {
	var req types.OrgCollectionsReq
	req.Namespace = ctx.Param("namespace")
	req.CurrentUser = httpbase.GetCurrentUser(ctx)

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Page = page
	req.PageSize = per
	datasets, total, err := h.colc.OrgCollections(ctx, &req)
	if err != nil {
		slog.Error("Failed to get org collections", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"message": "OK",
		"data":    datasets,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}
