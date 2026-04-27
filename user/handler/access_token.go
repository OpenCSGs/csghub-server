package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"opencsg.com/csghub-server/common/errorx"

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
	sc, err := apicomponent.NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	return &AccessTokenHandler{
		c:  ac,
		sc: sc,
	}, nil
}

type AccessTokenHandler struct {
	c  component.AccessTokenComponent
	sc apicomponent.SensitiveComponent
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
	if req.Application == "" {
		req.Application = types.AccessTokenAppGit
	}

	req.Username = ctx.Param("username")
	if currentUser != req.Username {
		slog.ErrorContext(ctx.Request.Context(), "user can only create its own access token", slog.String("current_user", currentUser), slog.String("username", req.Username))
		httpbase.UnauthorizedError(ctx, errors.New("user can only create its own access token"))
		return
	}
	token, err := h.c.Create(ctx, &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create user access token", slog.String("user_name", req.Username), slog.Any("error", err))
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

	req.Application = types.AccessTokenApp(ctx.Param("app"))
	req.Username = currentUser
	req.TokenName = ctx.Param("token_name")
	token, err := h.c.Create(ctx, &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create user access token", slog.String("user_name", req.Username), slog.Any("error", err))
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
// @Failure      404  {object}  error "User or access token not found"
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
		slog.ErrorContext(ctx.Request.Context(), "user can only delete its own access token", slog.String("current_user", currentUser), slog.String("username", req.Username))
		httpbase.UnauthorizedError(ctx, errors.New("user can only delete its own access token"))
		return
	}
	req.TokenName = ctx.Param("token_name")
	err := h.c.Delete(ctx, &req)
	if err != nil {
		if errors.Is(err, errorx.ErrNotFound) {
			httpbase.NotFoundError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete user access token", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Delete user access token succeed")
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
// @Failure      404  {object}  error "Access token not found"
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
		if errors.Is(err, errorx.ErrNotFound) {
			httpbase.NotFoundError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete user access token", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Delete user access token succeed")
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
// @Failure      404  {object}  error "Access token not found"
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
			slog.ErrorContext(ctx.Request.Context(), "Failed to parse expired_at", slog.String("expired_at", paramExpiredAt), slog.Any("error", err))
			httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("cannot parse expired_at, please use format RFC3339, like 2006-01-02T15:04:05Z07:00"), nil))
			return
		}
	}

	refreshReq := types.RefreshTokenReq{
		Username:     currentUser,
		TokenName:    tokenName,
		App:          app,
		NewExpiredAt: expiredAt,
	}

	resp, err := h.c.RefreshToken(ctx, &refreshReq)
	if err != nil {
		if errors.Is(err, errorx.ErrNotFound) {
			httpbase.NotFoundError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to refresh user access token", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.InfoContext(ctx.Request.Context(), "refresh user access token succeed", slog.String("current_user", currentUser),
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
		if errors.Is(err, errorx.ErrNotFound) {
			httpbase.NotFoundError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to check user access token", slog.Any("error", err), slog.Any("req", req))
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
	req := &types.GetAccessTokenRequest{
		Username:    currentUser,
		Application: types.AccessTokenApp(app),
	}
	resp, err := h.c.GetTokens(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get user access tokens", slog.Any("error", err), slog.Any("application", app), slog.String("current_user", currentUser))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

// GetUserFirstToken godoc
// @Security     ApiKey
// @Summary      Get or create first available access token for a user
// @Tags         Access token
// @Accept       json
// @Produce      json
// @Param        username path string true "username"
// @Param        current_user query string false "current user name"
// @Param        app query string false "application" Enums(git,starship)
// @Param        token_name query string false "token name"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /user/{username}/tokens/first [get]
func (h *AccessTokenHandler) GetOrCreateFirstAvaiTokens(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	app := ctx.Query("app")
	tokenName := ctx.Query("token_name")
	if app == "" || tokenName == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("app and tokenName query parameters are required"), nil))
		return
	}
	resp, err := h.c.GetOrCreateFirstAvaiToken(ctx, currentUser, app, tokenName)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get user access tokens", slog.Any("error", err), slog.Any("application", app), slog.String("current_user", currentUser))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

// CreateAPIKey godoc
// @Security     ApiKey
// @Summary      Create an API key for an organization or user
// @Tags         API Key
// @Accept       json
// @Produce      json
// @Param        uuid path string true "organization or user namespace uuid"
// @Param        body body types.CreateAPIKeyRequest true "body"
// @Success      200  {object}  types.Response{data=database.AccessToken} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  error "Forbidden - user is not org admin"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /namespaces/{uuid}/apikeys [post]
func (h *AccessTokenHandler) CreateAPIKey(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	userUUID := httpbase.GetCurrentUserUUID(ctx)
	nsUUID := ctx.Param("uuid")
	if nsUUID == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("namespace uuid is required"), nil))
		return
	}

	var req types.CreateAPIKeyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	apikeyReq := types.CreateUserTokenRequest{
		Username:    currentUser,
		OpUUID:      userUUID,
		NSUUID:      nsUUID,
		TokenName:   req.KeyName,
		Application: types.AccessTokenAppAIGateway,
		QuotaType:   req.QuotaType,
		ValueType:   req.ValueType,
		Quota:       req.Quota,
	}

	if req.ExpiredAt != nil {
		apikeyReq.ExpiredAt = *req.ExpiredAt
	}

	key, err := h.c.Create(ctx, &apikeyReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to create org API key", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, key)
}

// GetAPIKeys godoc
// @Security     ApiKey
// @Summary      Get all API keys for an organization or user
// @Tags         API Key
// @Accept       json
// @Produce      json
// @Param        uuid path string true "organization or user namespace uuid"
// @Success      200  {object}  types.Response{data=[]types.CheckAccessTokenResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /namespaces/{uuid}/apikeys [get]
func (h *AccessTokenHandler) GetAPIKeys(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	userUUID := httpbase.GetCurrentUserUUID(ctx)

	uuid := ctx.Param("uuid")
	if uuid == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("organization uuid is required"), nil))
		return
	}

	req := &types.GetAccessTokenRequest{
		Username:    currentUser,
		OpUUID:      userUUID,
		NSUUID:      uuid,
		Application: types.AccessTokenAppAIGateway,
	}

	apikeys, err := h.c.GetTokens(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get org API keys", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, apikeys)
}

// UpdateAPIKey godoc
// @Security     ApiKey
// @Summary      Update an API key for an organization or user
// @Tags         API Key
// @Accept       json
// @Produce      json
// @Param        uuid path string true "organization or user namespace uuid"
// @Param        id path string true "API key id"
// @Param        body body types.UpdateAPIKeyRequest true "body"
// @Success      200  {object}  types.Response{data=database.AccessToken} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  error "Forbidden - user is not org admin"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /namespaces/{uuid}/apikeys/{id} [put]
func (h *AccessTokenHandler) UpdateAPIKey(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	userUUID := httpbase.GetCurrentUserUUID(ctx)

	nsUUID := ctx.Param("uuid")
	if nsUUID == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("uuid is required"), nil))
		return
	}
	id := ctx.Param("id")
	if id == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("id is required"), nil))
		return
	}
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(err, nil))
		return
	}

	var req types.UpdateAPIKeyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	if req.Quota != nil {
		if *req.Quota < 0 {
			httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("quota must be non-negative"), nil))
			return
		}
	}

	req.ID = idInt
	req.CurrentUser = currentUser
	req.OpUUID = userUUID
	req.NSUUID = nsUUID

	token, err := h.c.Update(ctx, &req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to update API key", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, token)
}

// DeleteAPIKey godoc
// @Security     ApiKey
// @Summary      Delete an API key for an organization or user
// @Tags         API Key
// @Accept       json
// @Produce      json
// @Param        id path string true "API key id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  error "Forbidden - user is not org admin"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /namespace/{uuid}/apikeys/{id} [delete]
func (h *AccessTokenHandler) DeleteAPIKey(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	userUUID := httpbase.GetCurrentUserUUID(ctx)

	nsUUID := ctx.Param("uuid")
	if nsUUID == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("organization uuid is required"), nil))
		return
	}
	id := ctx.Param("id")
	if id == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("api_key_id is required"), nil))
		return
	}
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(err, nil))
		return
	}

	req := &types.DeleteUserTokenRequest{
		Username:    currentUser,
		OpUUID:      userUUID,
		NSUUID:      nsUUID,
		ID:          idInt,
		Application: types.AccessTokenAppAIGateway,
	}

	err = h.c.Delete(ctx, req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete API key", slog.String("org_uuid", nsUUID), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// GetAPIKeyQuotas godoc
// @Security     ApiKey
// @Summary      Get all quotas for an API key
// @Tags         Access token
// @Accept       json
// @Produce      json
// @Param        token_value path string true "API key"
// @Success      200  {object}  types.Response{data=[]database.AccountAccessTokenQuota} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /token/{token_value}/quotas [get]
func (h *AccessTokenHandler) GetAPIKeyQuotas(ctx *gin.Context) {
	apiKey := ctx.Param("token_value")
	if apiKey == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("api key is required"), nil))
		return
	}

	quotas, err := h.c.GetAPIKeyQuotas(ctx.Request.Context(), apiKey)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get API key quotas", slog.String("api_key", apiKey), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, quotas)
}

// GetBuiltinKeys godoc
// @Security     ApiKey
// @Summary      Get or create builtin API key for an organization or user namespace
// @Tags         API Key
// @Accept       json
// @Produce      json
// @Param        uuid path string true "organization or user namespace uuid"
// @Success      200  {object}  types.Response{data=types.CheckAccessTokenResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /namespaces/{uuid}/apikeys/builtin [get]
func (h *AccessTokenHandler) GetBuiltinKeys(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	userUUID := httpbase.GetCurrentUserUUID(ctx)

	nsUUID := ctx.Param("uuid")
	if nsUUID == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("namespace uuid is required"), nil))
		return
	}

	req := &types.GetAccessTokenRequest{
		Username:    currentUser,
		OpUUID:      userUUID,
		NSUUID:      nsUUID,
		Application: types.AccessTokenAppAIGateway,
	}

	key, err := h.c.GetOrCreateBuiltinAPIKey(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get or create builtin API key", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, key)
}

// RefreshBuiltinKey godoc
// @Security     ApiKey
// @Summary      Refresh builtin API key for an organization or user namespace
// @Tags         API Key
// @Accept       json
// @Produce      json
// @Param        uuid path string true "organization or user namespace uuid"
// @Success      200  {object}  types.Response{data=types.CheckAccessTokenResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      403  {object}  error "Forbidden - user is not org admin"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /namespaces/{uuid}/apikeys/builtin/refresh [put]
func (h *AccessTokenHandler) RefreshBuiltinKey(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	userUUID := httpbase.GetCurrentUserUUID(ctx)

	nsUUID := ctx.Param("uuid")
	if nsUUID == "" {
		httpbase.BadRequestWithExt(ctx, errorx.ReqParamInvalid(errors.New("namespace uuid is required"), nil))
		return
	}

	req := &types.RefreshTokenReq{
		NSUUID:    nsUUID,
		OpUUID:    userUUID,
		Username:  currentUser,
		TokenName: string(types.AccessTokenTypeBuiltIn),
		App:       string(types.AccessTokenAppAIGateway),
	}

	token, err := h.c.RefreshToken(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to refresh builtin API key", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, token)
}
