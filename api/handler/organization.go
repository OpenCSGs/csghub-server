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
	pc, err := component.NewPromptComponent(config)
	if err != nil {
		return nil, err
	}
	mcp, err := component.NewMCPServerComponent(config)
	if err != nil {
		return nil, err
	}
	return &OrganizationHandler{
		space:      sc,
		code:       cc,
		model:      mc,
		dataset:    dsc,
		collection: colc,
		prompt:     pc,
		mcp:        mcp,
	}, nil
}

type OrganizationHandler struct {
	space      component.SpaceComponent
	code       component.CodeComponent
	model      component.ModelComponent
	dataset    component.DatasetComponent
	collection component.CollectionComponent
	prompt     component.PromptComponent
	mcp        component.MCPServerComponent
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
	models, total, err := h.model.OrgModels(ctx.Request.Context(), &req)
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
	datasets, total, err := h.dataset.OrgDatasets(ctx.Request.Context(), &req)
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
	datasets, total, err := h.code.OrgCodes(ctx.Request.Context(), &req)
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
	datasets, total, err := h.space.OrgSpaces(ctx.Request.Context(), &req)
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
	datasets, total, err := h.collection.OrgCollections(ctx.Request.Context(), &req)
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

// GetOrganizationPrompts godoc
// @Security     ApiKey
// @Summary      Get organization prompts
// @Description  get organization prompts
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        namespace path string true "org name"
// @Param        current_user query string true "current user name"
// @Param        per query int false "page size"
// @Param        page query int false "current page number"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.PromptRes,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/prompts [get]
func (h *OrganizationHandler) Prompts(ctx *gin.Context) {
	var req types.OrgPromptsReq
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
	prompts, total, err := h.prompt.OrgPrompts(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("Failed to get org prompts", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"message": "OK",
		"data":    prompts,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// GetOrganizationMCPs godoc
// @Security     ApiKey
// @Summary      Get organization mcp servers
// @Description  get organization mcp servers
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        namespace path string true "org name"
// @Param        current_user query string true "current user name"
// @Param        per query int false "page size"
// @Param        page query int false "current page number"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.MCPServer,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/mcps [get]
func (h *OrganizationHandler) MCPServers(ctx *gin.Context) {
	var req types.OrgMCPsReq
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
	data, total, err := h.mcp.OrgMCPServers(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("Failed to get org mcp servers", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  data,
		"total": total,
	}
	httpbase.OK(ctx, respData)
}
