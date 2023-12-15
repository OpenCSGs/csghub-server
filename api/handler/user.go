package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/starhub-server/api/httpbase"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/common/utils/common"
	"opencsg.com/starhub-server/component"
)

func NewUserHandler(config *config.Config) (*UserHandler, error) {
	uc, err := component.NewUserComponent(config)
	if err != nil {
		return nil, err
	}
	return &UserHandler{
		c: uc,
	}, nil
}

type UserHandler struct {
	c *component.UserComponent
}

func (h *UserHandler) Create(ctx *gin.Context) {
	var req *types.CreateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	slog.Debug("Creating user", slog.Any("req", req))
	user, err := h.c.Create(ctx, req)
	if err != nil {
		slog.Error("Failed to create user", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Create user succeed", slog.String("user", user.Username))
	httpbase.OK(ctx, user)
}

func (h *UserHandler) Update(ctx *gin.Context) {
	var req *types.UpdateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	user, err := h.c.Update(ctx, req)
	if err != nil {
		slog.Error("Failed to update user", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Update user succeed", slog.String("user", user.Username))
	httpbase.OK(ctx, user)
}

func (h *UserHandler) Datasets(ctx *gin.Context) {
	var req types.UserDatasetsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = ctx.Query("current_user")
	req.Page = page
	req.PageSize = per
	ds, total, err := h.c.Datasets(ctx, &req)
	if err != nil {
		slog.Error("Failed to gat user datasets", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get user datasets succeed", slog.String("user", req.Owner))
	respData := gin.H{
		"message": "OK",
		"data":    ds,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}

func (h *UserHandler) Models(ctx *gin.Context) {
	var req types.UserDatasetsReq
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	req.Owner = ctx.Param("username")
	req.CurrentUser = ctx.Query("current_user")
	req.Page = page
	req.PageSize = per
	ms, total, err := h.c.Models(ctx, &req)
	if err != nil {
		slog.Error("Failed to gat user models", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Get user models succeed", slog.String("user", req.Owner))

	respData := gin.H{
		"message": "OK",
		"data":    ms,
		"total":   total,
	}
	ctx.JSON(http.StatusOK, respData)
}
