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

// CreateAccessToken godoc
// @Security     ApiKey
// @Summary      Create access token for a user
// @Description  create access token for a user
// @Tags         Access token
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        body body types.CreateUserTokenRequest true "body"
// @Success      200  {object}  types.Response{data=database.AccessToken} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/tokens [post]
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
		slog.Error("Failed to create user access token", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Create user access token succeed")
	httpbase.OK(ctx, token)
}

// DeleteAccessToken godoc
// @Security     ApiKey
// @Summary      Delete access token for a user
// @Description  delete access token for a user
// @Tags         Access token
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        token_name path string true "token_name"
// @Param        body body types.DeleteUserTokenRequest true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/tokens/{token_name} [delete]
func (h *AccessTokenHandler) Delete(ctx *gin.Context) {
	var req types.DeleteUserTokenRequest
	req.Username = ctx.Param("username")
	req.Name = ctx.Param("token_name")
	err := h.c.Delete(ctx, &req)
	if err != nil {
		slog.Error("Failed to delete user access token", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	slog.Info("Delete user access token succeed")
	httpbase.OK(ctx, nil)
}
