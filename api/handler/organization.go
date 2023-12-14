package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/api/httpbase"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/component"
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

func (h *OrganizationHandler) Update(ctx *gin.Context) {
	var req types.EditOrgReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	req.Path = ctx.Param("name")
	org, err := h.c.Update(ctx, &req)
	if err != nil {
		slog.Error("Failed to update organizations", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Delete organizations succeed", slog.String("org_name", org.Name))
	httpbase.OK(ctx, org)
}
