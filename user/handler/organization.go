package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	apicomponent "opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/user/component"
)

func NewOrganizationHandler(config *config.Config) (*OrganizationHandler, error) {
	oc, err := component.NewOrganizationComponent(config)
	if err != nil {
		return nil, err
	}
	return &OrganizationHandler{
		c:  oc,
		sc: apicomponent.NewSensitiveComponent(config),
	}, nil
}

type OrganizationHandler struct {
	c  *component.OrganizationComponent
	sc apicomponent.SensitiveChecker
}

// CreateOrganization godoc
// @Security     ApiKey
// @Summary      Create a new organization
// @Description  create a new organization
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        current_user query string false "the op user"
// @param        body body types.CreateOrgReq true "body"
// @Success      200  {object}  types.Response{data=types.Organization} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organizations [post]
func (h *OrganizationHandler) Create(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req types.CreateOrgReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var err error
	_, err = h.sc.CheckRequest(ctx, &req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	req.Username = currentUser
	org, err := h.c.Create(ctx, &req)
	if err != nil {
		slog.Error("Failed to create organization", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Create organization succeed", slog.String("org_path", org.Name))
	httpbase.OK(ctx, org)
}

// GetOrganization godoc
// @Security     ApiKey
// @Summary      Get organization info
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        current_user query string false "the op user"
// @param        namespace path string true "namespace"
// @Success      200  {object}  types.Response{data=types.Organization} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace} [get]
func (h *OrganizationHandler) Get(ctx *gin.Context) {
	orgName := ctx.Param("namespace")
	if len(orgName) == 0 {
		httpbase.BadRequest(ctx, "organization name is empty")
		return
	}
	org, err := h.c.Get(ctx, orgName)
	if err != nil {
		slog.Error("Failed to get organization", slog.Any("error", err), slog.String("org_path", orgName))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get organization succeed", slog.String("org_path", org.Name))
	httpbase.OK(ctx, org)
}

// GetOrganizations godoc
// @Security     ApiKey
// @Summary      Get organizations
// @Description  get organizations
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{data=[]types.Organization} "OK"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organizations [get]
func (h *OrganizationHandler) Index(ctx *gin.Context) {
	username := httpbase.GetCurrentUser(ctx)
	orgs, err := h.c.Index(ctx, username)
	if err != nil {
		slog.Error("Failed to get organizations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Get organizations succeed")
	httpbase.OK(ctx, orgs)
}

// DeleteOrganization godoc
// @Security     ApiKey
// @Summary      Delete organization
// @Description  delete organization
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        current_user query string false "the op user"
// @Param        body body types.DeleteOrgReq true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace} [delete]
func (h *OrganizationHandler) Delete(ctx *gin.Context) {
	var req types.DeleteOrgReq
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	req.CurrentUser = currentUser
	req.Name = ctx.Param("namespace")
	err := h.c.Delete(ctx, &req)
	if err != nil {
		slog.Error("Failed to delete organizations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Delete organizations succeed", slog.String("org_name", req.Name))
	httpbase.OK(ctx, nil)
}

// UpdateOrganization godoc
// @Security     ApiKey
// @Summary      Update organization
// @Description  update organization
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        current_user query string false "the op user"
// @Param        body body types.EditOrgReq true "body"
// @Success      200  {object}  types.Response{data=database.Organization} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace} [put]
func (h *OrganizationHandler) Update(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req types.EditOrgReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var err error
	_, err = h.sc.CheckRequest(ctx, &req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}
	req.CurrentUser = currentUser
	req.Name = ctx.Param("namespace")
	org, err := h.c.Update(ctx, &req)
	if err != nil {
		slog.Error("Failed to update organizations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update organizations succeed", slog.String("org_name", org.Nickname))
	httpbase.OK(ctx, org)
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
	models, total, err := h.c.Models(ctx, &req)
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
	datasets, total, err := h.c.Datasets(ctx, &req)
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
	datasets, total, err := h.c.Codes(ctx, &req)
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
	datasets, total, err := h.c.Spaces(ctx, &req)
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
	datasets, total, err := h.c.Collections(ctx, &req)
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
