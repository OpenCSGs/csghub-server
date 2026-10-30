package handler

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/user/component"
)

func NewJWTHandler(config *config.Config) (*JWTHandler, error) {
	return &JWTHandler{
		c: component.NewJwtComponent(config.JWT.SigningKey, config.JWT.ValidHour),
	}, nil
}

type JWTHandler struct {
	c component.JwtComponent
}

// CreateJWTToken   godoc
// @Security     ApiKey
// @Summary      generate jwt token for user
// @Tags         JWT
// @Accept       json
// @Produce      json
// @Param        body body types.CreateJWTReq true "body"
// @Success      200  {object}  types.CreateJWTResp "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /jwt/token [post]
func (h *JWTHandler) Create(ctx *gin.Context) {
	var req types.CreateJWTReq
	if err := ctx.ShouldBind(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	slog.InfoContext(ctx.Request.Context(), "Create JWT token", slog.String("uuid", req.UUID), slog.Any("orgs", req.Organizations))
	claims, signed, err := h.c.GenerateToken(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to generate JWT token", slog.Any("error", err))
		if errors.Is(err, errorx.ErrInvalidJWT) || errors.Is(err, errorx.ErrNeedOldToken) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}

	expireTime := claims.ExpiresAt
	resp := &types.CreateJWTResp{
		Token:    signed,
		ExpireAt: expireTime.Time,
	}

	httpbase.OK(ctx, resp)
}

// VerifyJWTToken   godoc
// @Security     ApiKey
// @Summary      verify jwt token and return user info
// @Tags         JWT
// @Accept       json
// @Produce      json
// @Param        token path string true "token"
// @Success      200  {object}  types.User "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /jwt/{token} [get]
func (h *JWTHandler) Verify(ctx *gin.Context) {
	token := ctx.Param("token")
	user, err := h.c.ParseToken(ctx.Request.Context(), token)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to verify JWT token", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, user)
}

// RefreshAdmin refreshes only the authenticated administrator's current JWT session.
// @Security ApiKey
// @Summary Refresh the current administrator JWT
// @Description Independently requires administrator membership. UUID and old_token must match the authenticated caller; the existing edition-specific expiry policy is preserved.
// @Tags Admin Common
// @Accept json
// @Produce json
// @Param body body types.RefreshJWTReq true "Current user UUID and existing JWT"
// @Success 200 {object} types.Response{data=types.CreateJWTResp}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/common/jwt/token [post]
func (h *JWTHandler) RefreshAdmin(ctx *gin.Context) {
	uuid := httpbase.GetCurrentUserUUID(ctx)
	authorization := ctx.GetHeader("Authorization")
	if uuid == "" || httpbase.GetAuthType(ctx) != httpbase.AuthTypeJwt || !strings.HasPrefix(authorization, "Bearer ") {
		httpbase.UnauthorizedError(ctx, errorx.ErrInvalidJWT)
		return
	}
	var req types.RefreshJWTReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	if req.UUID != uuid || req.OldToken != strings.TrimPrefix(authorization, "Bearer ") {
		httpbase.ForbiddenError(ctx, errorx.ErrInvalidJWT)
		return
	}
	claims, signed, err := h.c.RefreshToken(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrInvalidJWT) || errors.Is(err, errorx.ErrNeedOldToken) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, &types.CreateJWTResp{Token: signed, ExpireAt: claims.ExpiresAt.Time})
}
