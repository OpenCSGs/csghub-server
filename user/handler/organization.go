package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
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
	ov, err := component.NewOrganizationVerifyComponent(config)
	if err != nil {
		return nil, err
	}
	return &OrganizationHandler{
		c:  oc,
		sc: sc,
		ov: ov,
	}, nil
}

type OrganizationHandler struct {
	c  component.OrganizationComponent
	sc apicomponent.SensitiveComponent
	ov component.OrganizationVerifyComponent
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
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	var err error
	_, err = h.sc.CheckRequestV2(ctx, &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("sensitive check failed: %w", err))
		return
	}

	req.Username = currentUser
	org, err := h.c.Create(ctx, &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create organization", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Create organization succeed", slog.String("org_path", org.Name))
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
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("organization name is empty"), nil))
		return
	}
	org, err := h.c.Get(ctx, orgName)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get organization", slog.Any("error", err), slog.String("org_path", orgName))
		if errors.Is(err, errorx.ErrDatabaseNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			httpbase.ServerError(ctx, err)
		}
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Get organization succeed", slog.String("org_path", org.Name))
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
	orgType := ctx.Query("org_type")
	verifyStatus := ctx.Query("verify_status")
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get per and page", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	orgs, total, err := h.c.Index(ctx, username, search, per, page, orgType, verifyStatus)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get organizations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  orgs,
		"total": total,
	}

	slog.InfoContext(ctx.Request.Context(), "Get organizations succeed", slog.String("username", username), slog.String("search", search), slog.Int("per", per), slog.Int("page", page))
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
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete organizations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Delete organizations succeed", slog.String("org_name", req.Name))
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
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	var err error
	_, err = h.sc.CheckRequestV2(ctx, &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check sensitive request", slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("sensitive check failed: %w", err))
		return
	}
	req.CurrentUser = currentUser
	req.Name = ctx.Param("namespace")
	org, err := h.c.Update(ctx, &req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to update organizations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Update organizations succeed", slog.String("org_name", org.Nickname))
	httpbase.OK(ctx, org)
}

// CreateVerify godoc
// @Security     ApiKey
// @Summary      Create organization verification
// @Description  create a new organization verification request
// @Tags         OrganizationVerify
// @Accept       json
// @Produce      json
// @Param        body body types.OrgVerifyReq true "Organization verification request body"
// @Success      200  {object}  types.Response{data=database.OrganizationVerify} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/verify [post]
func (h *OrganizationHandler) CreateVerify(ctx *gin.Context) {
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	currentUser := httpbase.GetCurrentUser(ctx)
	var req types.OrgVerifyReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	req.Username = currentUser
	req.UserUUID = currentUserUUID
	orgVerify, err := h.ov.Create(ctx, &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create organization Verify", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Create organization Verify succeed", slog.String("company_name", orgVerify.CompanyName))
	httpbase.OK(ctx, orgVerify)
}

// UpdateVerify godoc
// @Security     ApiKey
// @Summary      Update organization verification
// @Description  update organization verification status (approved or rejected)
// @Tags         OrganizationVerify
// @Accept       json
// @Produce      json
// @Param        id     path  int    true  "verification ID"
// @Param        body body types.OrgVerifyStatusReq true "Update verification request body"
// @Success      200  {object}  types.Response{data=database.OrganizationVerify} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/verify/{id} [put]
func (h *OrganizationHandler) UpdateVerify(ctx *gin.Context) {
	vID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	var req types.OrgVerifyStatusReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	if req.Status != types.VerifyStatusRejected && req.Status != types.VerifyStatusApproved {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", slog.String("err", "Not allowed status"))
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("not allowed status"), nil))
	}

	if req.Status == types.VerifyStatusRejected && req.Reason == "" {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", slog.String("err", "rejected need reason"))
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("rejected need reason"), nil))
	}

	orgVerify, err := h.ov.Update(ctx, vID, req.Status, req.Reason)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update organization Verify", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.ErrorContext(ctx.Request.Context(), "update organization Verify succeed", slog.String("company_name", orgVerify.CompanyName))
	httpbase.OK(ctx, orgVerify)
}

// GetVerify godoc
// @Security     ApiKey
// @Summary      Get organization verification
// @Description  get organization verification info by organization ID
// @Tags         OrganizationVerify
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Success      200  {object}  types.Response{data=database.OrganizationVerify} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/verify/{namespace} [get]
func (h *OrganizationHandler) GetVerify(ctx *gin.Context) {
	path := ctx.Param("namespace")
	orgVerify, err := h.ov.Get(ctx, path)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get organization Verify", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, orgVerify)
}
