package handler

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
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

func (h *MemberHandler) Index(ctx *gin.Context) {
	members, err := h.c.Index(ctx)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, members)
}

func (h *MemberHandler) Update(ctx *gin.Context) {
	member, err := h.c.Update(ctx)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, member)
}

// Create   godoc
// @Security     ApiKey
// @Summary      Create new membership between org and user
// @Description  user will be added to org with a role
// @Tags         Member
// @Accept       json
// @Produce      json
// @Param        name path string true "org name"
// @Param        body body handler.Create.addMemberRequest true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organizations/{name}/members [post]
func (h *MemberHandler) Create(ctx *gin.Context) {
	type addMemberRequest struct {
		//name of user will be added to the org as a member
		User string `json:"user"`
		Role string `json:"role"`
		//name of the operator
		OpUser string `json:"op_user"`
	}
	req := new(addMemberRequest)
	if err := ctx.ShouldBindJSON(req); err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("failed to unmarshal request body,caused by:%w", err).Error())
		return
	}
	org := ctx.Param("name")
	err := h.c.AddMember(ctx, org, req.User, req.OpUser, req.Role)
	if err != nil {
		slog.ErrorContext(ctx, "create member fail", slog.Any("error", err),
			slog.Group("request",
				slog.String("org", org), slog.String("user", req.User), slog.String("role", req.Role), slog.String("op_user", req.OpUser),
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
// @Param        name path string true "org name"
// @Param        username path string true "user name"
// @Param        body body handler.Delete.removeMemberRequest true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /organizations/{name}/members/:username [delete]
func (h *MemberHandler) Delete(ctx *gin.Context) {
	type removeMemberRequest struct {
		Role string `json:"role"`
		//name of the operator
		OpUser string `json:"op_user"`
	}
	req := new(removeMemberRequest)
	if err := ctx.ShouldBindJSON(req); err != nil {
		httpbase.BadRequest(ctx, fmt.Errorf("failed to unmarshal request body,caused by:%w", err).Error())
		return
	}
	org := ctx.Param("name")
	userName := ctx.Param("username")
	err := h.c.Delete(ctx, org, userName, req.OpUser, req.Role)
	if err != nil {
		slog.ErrorContext(ctx, "delete member fail", slog.Any("error", err),
			slog.Group("request",
				slog.String("org", org), slog.String("username", userName), slog.String("role", req.Role), slog.String("op_user", req.OpUser),
			),
		)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}
