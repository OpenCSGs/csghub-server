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
		httpbase.BadRequestWithExt(ctx, errorx.ErrSensitiveInfoNotAllowed)
		return
	}

	req.Username = currentUser
	org, err := h.c.Create(ctx, &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create organization", slog.Any("error", err))
		if errors.Is(err, errorx.ErrNamespaceAlreadyExists) || errors.Is(err, errorx.ErrTagIDsNotExist) {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
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

// GetOrganizationByUUID godoc
// @Security     ApiKey
// @Summary      Get organization by UUID
// @Description  get organization by UUID
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        uuid path string true "organization uuid"
// @Success      200  {object}  types.Response{data=types.Organization} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/uuid/{uuid} [get]
func (h *OrganizationHandler) GetByUUID(ctx *gin.Context) {
	uuid := ctx.Param("uuid")
	if len(uuid) == 0 {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("organization uuid is empty"), nil))
		return
	}
	org, err := h.c.GetByUUID(ctx, uuid)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get organization by UUID", slog.Any("error", err), slog.String("org_uuid", uuid))
		if errors.Is(err, errorx.ErrDatabaseNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			httpbase.ServerError(ctx, err)
		}
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Get organization by UUID succeed", slog.String("org_uuid", uuid))
	httpbase.OK(ctx, org)
}

// GetOrganizations godoc
// @Summary      Get all organizations
// @Description  get all organizations, no authentication required
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        search query string false "search keyword"
// @Param        org_type query string false "org type filter"
// @Param        verify_status query string false "verify status filter"
// @Param        tag query string false "filter by tag name"
// @Param        per query int false "page size"
// @Param        page query int false "page number"
// @Success      200  {object}  types.Response{data=[]types.Organization} "OK"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organizations [get]
func (h *OrganizationHandler) Index(ctx *gin.Context) {
	search := ctx.Query("search")
	orgType := ctx.Query("org_type")
	verifyStatus := ctx.Query("verify_status")
	tag := ctx.Query("tag")
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get per and page", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	orgs, total, err := h.c.Index(ctx, search, per, page, orgType, verifyStatus, tag)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get organizations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  orgs,
		"total": total,
	}

	slog.InfoContext(ctx.Request.Context(), "Get all organizations succeed", slog.String("search", search), slog.Int("per", per), slog.Int("page", page))
	httpbase.OK(ctx, respData)
}

// ListUserOrganizations godoc
// @Security     ApiKey
// @Summary      Get organizations the user belongs to
// @Description  get organizations the specified user belongs to, with optional role filter (all, owner, write, admin)
// @Tags         Organization
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        search query string false "search keyword"
// @Param        org_type query string false "org type filter"
// @Param        verify_status query string false "verify status filter"
// @Param        role query string false "role filter: all (any member), owner, write, admin"
// @Param        tag query string false "filter by tag name"
// @Param        per query int false "page size"
// @Param        page query int false "page number"
// @Success      200  {object}  types.Response{data=[]types.Organization} "OK"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/organizations [get]
func (h *OrganizationHandler) ListUserOrgs(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get per and page", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	req := &types.ListUserOrgsReq{
		Username:     ctx.Param("username"),
		Search:       ctx.Query("search"),
		OrgType:      ctx.Query("org_type"),
		VerifyStatus: ctx.Query("verify_status"),
		Role:         ctx.Query("role"),
		Tag:          ctx.Query("tag"),
		Per:          per,
		Page:         page,
	}
	orgs, total, err := h.c.ListUserOrgs(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get user organizations", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  orgs,
		"total": total,
	}

	slog.InfoContext(ctx.Request.Context(), "Get user organizations succeed", slog.String("username", req.Username))
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
		httpbase.BadRequestWithExt(ctx, errorx.ErrSensitiveInfoNotAllowed)
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
		if errors.Is(err, errorx.ErrTagIDsNotExist) {
			httpbase.BadRequestWithExt(ctx, err)
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
