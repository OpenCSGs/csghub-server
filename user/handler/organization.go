package handler

import (
	"errors"
	"fmt"
	"log/slog"

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
	sc, err := apicomponent.NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	return &OrganizationHandler{
		c:  oc,
		sc: sc,
	}, nil
}

type OrganizationHandler struct {
	c  component.OrganizationComponent
	sc apicomponent.SensitiveComponent
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
	_, err = h.sc.CheckRequestV2(ctx, &req)
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
	search := ctx.Query("search")
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Failed to get per and page", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	orgs, total, err := h.c.Index(ctx, username, search, per, page)
	if err != nil {
		slog.Error("Failed to get organizations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  orgs,
		"total": total,
	}

	slog.Info("Get organizations succeed", slog.String("username", username), slog.String("search", search), slog.Int("per", per), slog.Int("page", page))
	httpbase.OK(ctx, respData)
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
	_, err = h.sc.CheckRequestV2(ctx, &req)
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
