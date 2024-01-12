package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewAccessTokenHandler(config *config.Config) (*AccessTokenHandler, error) {
	ac, err := component.NewAccessTokenComponent(config)
	if err != nil {
		return nil, err
	}
	return &AccessTokenHandler{
		c: ac,
	}, nil
}

type AccessTokenHandler struct {
	c *component.AccessTokenComponent
}

func (h *AccessTokenHandler) Create(ctx *gin.Context) {
	var req types.CreateUserTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	req.Username = ctx.Param("username")
	token, err := h.c.Create(ctx, &req)
	if err != nil {
		if err != nil {
			slog.Error("Failed to create user access token", slog.Any("error", err))
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
	}

	slog.Info("Create user access token succeed")
	httpbase.OK(ctx, token)
}

func (h *AccessTokenHandler) Delete(ctx *gin.Context) {
	var req types.DeleteUserTokenRequest
	req.Username = ctx.Param("username")
	req.Name = ctx.Param("token_name")
	err := h.c.Delete(ctx, &req)
	if err != nil {
		if err != nil {
			slog.Error("Failed to delete user access token", slog.Any("error", err))
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
	}

	slog.Info("Delete user access token succeed")
	httpbase.OK(ctx, nil)
}
