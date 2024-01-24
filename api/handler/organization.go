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
	oc, err := component.NewOrganizationComponent(config)
	if err != nil {
		return nil, err
	}
	return &OrganizationHandler{
		c: oc,
	}, nil
}

type OrganizationHandler struct {
	c *component.OrganizationComponent
}

// CreateOrganization godoc
// @Security     ApiKey
// @Summary      Create a new organization
// @Description  create a new organization
// @Tags         Organization
// @Accept       json
// @Produce      json
// @param        body body types.CreateOrgReq true "body"
// @Success      200  {object}  types.Response{data=database.Organization} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organizations [post]
func (h *OrganizationHandler) Create(ctx *gin.Context) {
	var req types.CreateOrgReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	org, err := h.c.Create(ctx, &req)
	if err != nil {
		slog.Error("Failed to create organization", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Create organization succeed", slog.String("org_path", org.Path))
	httpbase.OK(ctx, org)
}

// GetOrganizations godoc
// @Security     ApiKey
// @Summary      Get organizations
// @Description  get organizations
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.ResponseWithTotal{data=[]database.Organization,total=int} "OK"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organizations [get]
func (h *OrganizationHandler) Index(ctx *gin.Context) {
	username := ctx.Query("username")
	orgs, err := h.c.Index(ctx, username)
	if err != nil {
		slog.Error("Failed to get organizations", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
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
// @Param        name path string true "name"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organizations/{name} [delete]
func (h *OrganizationHandler) Delete(ctx *gin.Context) {
	name := ctx.Param("name")
	err := h.c.Delete(ctx, name)
	if err != nil {
		slog.Error("Failed to delete organizations", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Delete organizations succeed", slog.String("org_name", name))
	httpbase.OK(ctx, nil)
}

// UpdateOrganization godoc
// @Security     ApiKey
// @Summary      Update organization
// @Description  update organization
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        name path string true "name"
// @Param        body body types.EditOrgReq true "body"
// @Success      200  {object}  types.Response{data=database.Organization} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organizations/{name} [put]
func (h *OrganizationHandler) Update(ctx *gin.Context) {
	var req types.EditOrgReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req.Name = ctx.Param("name")
	org, err := h.c.Update(ctx, &req)
	if err != nil {
		slog.Error("Failed to update organizations", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Update organizations succeed", slog.String("org_name", org.Name))
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
// @Success      200  {object}  types.ResponseWithTotal{data=[]database.Model,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/models [get]
func (h *OrganizationHandler) Models(ctx *gin.Context) {
	var req types.OrgModelsReq
	req.Namespace = ctx.Param("namespace")
	req.CurrentUser = ctx.Query("current_user")

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req.Page = page
	req.PageSize = per
	models, total, err := h.c.Models(ctx, &req)
	if err != nil {
		slog.Error("Failed to gat org models", slog.Any("error", err))
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
// @Success      200  {object}  types.ResponseWithTotal{data=[]database.Dataset,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/datasets [get]
func (h *OrganizationHandler) Datasets(ctx *gin.Context) {
	var req types.OrgDatasetsReq
	req.Namespace = ctx.Param("namespace")
	req.CurrentUser = ctx.Query("current_user")

	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req.Page = page
	req.PageSize = per
	datasets, total, err := h.c.Datasets(ctx, &req)
	if err != nil {
		slog.Error("Failed to gat org datasets", slog.Any("error", err))
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
