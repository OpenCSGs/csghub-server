package handler

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/user/component"
)

func NewJWTHandler(config *config.Config) (*JWTHandler, error) {
	return &JWTHandler{
		c: component.NewJwtComponent(config.JWT.SigningKey, config.JWT.ValidHour),
	}, nil
}

type JWTHandler struct {
	c *component.JwtComponent
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
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	claims, signed, err := h.c.GenerateToken(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to generate JWT token", slog.Any("error", err))
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
		slog.Error("failed to verify JWT token", slog.Any("error", err), slog.String("token", token))
		httpbase.ServerError(ctx, fmt.Errorf("failed to verify JWT token '%s': %w", token, err))
		return
	}

	httpbase.OK(ctx, user)
}
