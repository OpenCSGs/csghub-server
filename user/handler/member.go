package handler

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/user/component"
)

type MemberHandler struct {
	c *component.MemberComponent
}

func NewMemberHandler(config *config.Config) (*MemberHandler, error) {
	mc, err := component.NewMemberComponent(config)
	if err != nil {
		return nil, err
	}
	return &MemberHandler{
		c: mc,
	}, nil
}

// GetOrganizationMembers godoc
// @Security     ApiKey
// @Summary      Get organization members. Org member can get more details.
// @Tags         Member
// @Accept       json
// @Produce      json
// @Param        current_user query string false "the op user"
// @param        namespace path string true "namespace"
// @Param        per query int false "per" default(50)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=types.Member, total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/members [get]
func (h *MemberHandler) OrgMembers(ctx *gin.Context) {
	orgName := ctx.Param("namespace")
	if orgName == "" {
		httpbase.BadRequest(ctx, fmt.Errorf("org name is empty").Error())
		return
	}
	pageSize, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	members, total, err := h.c.OrgMembers(ctx.Request.Context(), orgName, currentUser, pageSize, page)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  members,
		"total": total,
	}
	httpbase.OK(ctx, respData)
}

// UpdateMember   godoc
// @Security     ApiKey
// @Summary      update user membership
// @Tags         Member
// @Accept       json
// @Produce      json
// @Param        namespace path string true "org name"
// @Param        username path string true "user name"
// @Param        current_user query string false "the op user"
// @Param        body body handler.Update.updateMemberRequest true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/members/{username} [put]
func (h *MemberHandler) Update(ctx *gin.Context) {
	type updateMemberRequest struct {
		OldRole string `json:"old_role" binding:"required"`
		NewRole string `json:"new_role" binding:"required"`
	}
	req := new(updateMemberRequest)
	if err := ctx.ShouldBindJSON(req); err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("failed to unmarshal request body,caused by:%w", err).Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	org := ctx.Param("namespace")
	userName := ctx.Param("username")
	err := h.c.ChangeMemberRole(ctx, org, userName, currentUser, req.OldRole, req.NewRole)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// Create   godoc
// @Security     ApiKey
// @Summary      Create new membership between org and user
// @Description  user will be added to org with a role
// @Tags         Member
// @Accept       json
// @Produce      json
// @Param        namespace path string true "org name"
// @Param        current_user query string false "the op user"
// @Param        body body handler.Create.addMemberRequest true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/members [post]
func (h *MemberHandler) Create(ctx *gin.Context) {
	type addMemberRequest struct {
		// name of user will be added to the org as a member
		Users string `json:"users" binding:"required" example:"user1,user2"`
		Role  string `json:"role" binding:"required"`
	}
	req := new(addMemberRequest)
	if err := ctx.ShouldBindJSON(req); err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("failed to unmarshal request body,caused by:%w", err).Error())
		return
	}
	users := strings.Split(req.Users, ",")
	if len(users) == 0 {
		httpbase.BadRequest(ctx, fmt.Errorf("user name is empty").Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	org := ctx.Param("namespace")
	err := h.c.AddMembers(ctx, org, users, currentUser, req.Role)
	if err != nil {
		slog.ErrorContext(ctx, "create member fail", slog.Any("error", err),
			slog.Group("request",
				slog.String("org", org), slog.String("user", req.Users), slog.String("role", req.Role), slog.String("op_user", currentUser),
			),
		)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// Delete   godoc
// @Security     ApiKey
// @Summary      Remove membership between org and user
// @Description  user's role will be remove from org
// @Tags         Member
// @Accept       json
// @Produce      json
// @Param        namespace path string true "org name"
// @Param        username path string true "user name"
// @Param        current_user query string false "the op user"
// @Param        body body handler.Delete.removeMemberRequest true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/members/{username} [delete]
func (h *MemberHandler) Delete(ctx *gin.Context) {
	type removeMemberRequest struct {
		Role string `json:"role" binding:"required"`
	}
	req := new(removeMemberRequest)
	if err := ctx.ShouldBindJSON(req); err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("failed to unmarshal request body,caused by:%w", err).Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	org := ctx.Param("namespace")
	userName := ctx.Param("username")
	err := h.c.Delete(ctx, org, userName, currentUser, req.Role)
	if err != nil {
		slog.ErrorContext(ctx, "delete member fail", slog.Any("error", err),
			slog.Group("request",
				slog.String("org", org), slog.String("username", userName), slog.String("role", req.Role), slog.String("op_user", currentUser),
			),
		)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// GetMemberRole   godoc
// @Security     ApiKey
// @Summary      Get user's role in an org
// @Tags         Member
// @Accept       json
// @Produce      json
// @Param        namespace path string true "org name"
// @Param        username path string true "user name"
// @Param        current_user query string false "the op user"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organization/{namespace}/members/{username} [get]
func (h *MemberHandler) GetMemberRole(ctx *gin.Context) {
	org := ctx.Param("namespace")
	userName := ctx.Param("username")
	// Assuming GetMemberRole returns a role (or similar) and an error
	role, err := h.c.GetMemberRole(ctx.Request.Context(), org, userName)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, role)
}
