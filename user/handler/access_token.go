package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	apicomponent "opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/user/component"
)

func NewAccessTokenHandler(config *config.Config) (*AccessTokenHandler, error) {
	ac, err := component.NewAccessTokenComponent(config)
	if err != nil {
		return nil, err
	}
	return &AccessTokenHandler{
		c:  ac,
		sc: apicomponent.NewSensitiveComponent(config),
	}, nil
}

type AccessTokenHandler struct {
	c  *component.AccessTokenComponent
	sc apicomponent.SensitiveChecker
}

// CreateAccessToken godoc
// @Security     ApiKey
// @Summary      [Deprecated: use POST:/token/{app}/{username} instead]
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
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var req types.CreateUserTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var err error
	_, err = h.sc.CheckRequest(ctx, &req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}
	if req.Application == "" {
		req.Application = types.AccessTokenAppGit
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

// CreateAppToken godoc
// @Security     ApiKey
// @Summary      Create access token for an special application
// @Tags         Access token
// @Accept       json
// @Produce      json
// @Param        token_name path string true "token name"
// @Param        app path string true "application" Enums(git,starship)
// @Param        current_user query string true "current user, the owner"
// @Param        body body types.CreateUserTokenRequest true "body"
// @Success      200  {object}  types.Response{data=database.AccessToken} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /token/{app}/{token_name} [post]
func (h *AccessTokenHandler) CreateAppToken(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var req types.CreateUserTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var err error
	_, err = h.sc.CheckRequest(ctx, &req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	req.Application = types.AccessTokenApp(ctx.Param("app"))
	req.Username = currentUser
	req.TokenName = ctx.Param("token_name")
	token, err := h.c.Create(ctx, &req)
	if err != nil {
		slog.Error("Failed to create user access token", slog.String("user_name", req.Username), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, token)
}

// Deprecated: use DeleteAppToken instead
// DeleteAccessToken godoc
// @Security     ApiKey
// @Summary      [Deprecated: use DELETE:/token/{app}/{token_name} instead]
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
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var req types.DeleteUserTokenRequest
	req.Username = ctx.Param("username")
	if currentUser != req.Username {
		slog.Error("user can only delete its own access token", slog.String("current_user", currentUser), slog.String("username", req.Username))
		httpbase.UnauthorizedError(ctx, errors.New("user can only delete its own access token"))
		return
	}
	req.TokenName = ctx.Param("token_name")
	err := h.c.Delete(ctx, &req)
	if err != nil {
		slog.Error("Failed to delete user access token", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Delete user access token succeed")
	httpbase.OK(ctx, nil)
}

// DeleteAppToken godoc
// @Security     ApiKey
// @Summary      Delete access token of a app
// @Tags         Access token
// @Accept       json
// @Produce      json
// @Param        app path string true "application" Enums(git,starship)
// @Param        token_name path string true "token_name"
// @Param        current_user query string true "current user, the owner"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /token/{app}/{token_name} [delete]
func (h *AccessTokenHandler) DeleteAppToken(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	app := ctx.Param("app")
	tokenName := ctx.Param("token_name")
	req := types.DeleteUserTokenRequest{
		Username:    currentUser,
		TokenName:   tokenName,
		Application: types.AccessTokenApp(app),
	}
	err := h.c.Delete(ctx, &req)
	if err != nil {
		slog.Error("Failed to delete user access token", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Delete user access token succeed")
	httpbase.OK(ctx, nil)
}

// RefreshToken godoc
// @Security     ApiKey
// @Summary      Refresh a access token for a user
// @Tags         Access token
// @Accept       json
// @Produce      json
// @Param        app path string true "application" Enums(git,starship)
// @Param        token_name path string true "token_name"
// @Param        current_user query string true "current user, the owner"
// @Param        expired_at query string false "new expire time, in format RFC3339, like 2006-01-02T15:04:05Z07:00"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /token/{app}/{token_name} [put]
func (h *AccessTokenHandler) Refresh(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	app := ctx.Param("app")
	tokenName := ctx.Param("token_name")
	var expiredAt time.Time
	var err error
	paramExpiredAt := ctx.Param("expired_at")
	if len(paramExpiredAt) > 0 {
		expiredAt, err = time.Parse(time.RFC3339, paramExpiredAt)
		if err != nil {
			slog.Error("Failed to parse expired_at", slog.String("expired_at", paramExpiredAt), slog.Any("error", err))
			httpbase.BadRequest(ctx, "cannot parse expired_at, please use format RFC3339, like 2006-01-02T15:04:05Z07:00")
			return
		}
	}
	resp, err := h.c.RefreshToken(ctx, currentUser, tokenName, app, expiredAt)
	if err != nil {
		slog.Error("Failed to refresh user access token", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("refresh user access token succeed", slog.String("current_user", currentUser),
		slog.String("app", app), slog.String("token_name", tokenName))
	httpbase.OK(ctx, resp)
}

// GetAccessToken godoc
// @Security     ApiKey
// @Summary      Get token and owner's detail by the token value
// @Tags         Access token
// @Accept       json
// @Produce      json
// @Param        token_value path string true "token_value"
// @Param        app query string false "application" Enums(git,starship)
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /token/{token_value} [get]
func (h *AccessTokenHandler) Get(ctx *gin.Context) {
	//!can not check current user here, because dont know user name when validate an access token
	/*
		currentUser := httpbase.GetCurrentUser(ctx)
		if currentUser == "" {
			httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
			return
		}
	*/
	var req types.CheckAccessTokenReq
	req.Token = ctx.Param("token_value")
	req.Application = ctx.Query("app")
	resp, err := h.c.Check(ctx, &req)
	if err != nil {
		slog.Error("Failed to check user access token", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

// GetUserTokens godoc
// @Security     ApiKey
// @Summary      Get all access tokens for a user
// @Tags         Access token
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        current_user query string false "current user name"
// @Param        app query string false "application" Enums(git,starship)
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/tokens [get]
func (h *AccessTokenHandler) GetUserTokens(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	app := ctx.Query("app")
	resp, err := h.c.GetTokens(ctx, currentUser, app)
	if err != nil {
		slog.Error("Failed to get user access tokens", slog.Any("error", err), slog.Any("application", app), slog.String("current_user", currentUser))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}
