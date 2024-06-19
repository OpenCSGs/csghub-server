package handler

import (
	"errors"
	"log/slog"

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
// @Param        current_user query string true "current user, the owner"
// @Param        body body types.CreateUserTokenRequest true "body"
// @Success      200  {object}  types.Response{data=database.AccessToken} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/tokens [post]
func (h *AccessTokenHandler) Create(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req types.CreateUserTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if req.Application == "" {
		req.Application = types.AccessTokenApplicationGit
	}

	req.Username = ctx.Param("username")
	if currentUser != req.Username {
		slog.Error("user can only create its own access token", slog.String("current_user", currentUser), slog.String("username", req.Username))
		httpbase.UnauthorizedError(ctx, errors.New("user can only create its own access token"))
		return
	}
	token, err := h.c.Create(ctx, &req)
	if err != nil {
		slog.Error("Failed to create user access token", slog.String("user_name", req.Username), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

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
// @Param        current_user query string true "current user, the owner"
// @Param        body body types.DeleteUserTokenRequest true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/tokens/{token_name} [delete]
func (h *AccessTokenHandler) Delete(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req types.DeleteUserTokenRequest
	req.Username = ctx.Param("username")
	if currentUser != req.Username {
		slog.Error("user can only delete its own access token", slog.String("current_user", currentUser), slog.String("username", req.Username))
		httpbase.UnauthorizedError(ctx, errors.New("user can only delete its own access token"))
		return
	}
	req.Name = ctx.Param("token_name")
	err := h.c.Delete(ctx, &req)
	if err != nil {
		slog.Error("Failed to delete user access token", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Delete user access token succeed")
	httpbase.OK(ctx, nil)
}
