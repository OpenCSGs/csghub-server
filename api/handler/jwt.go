package handler

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewJWTHandler(config *config.Config) (*JWTHandler, error) {
	return &JWTHandler{
		SigningKey: []byte(config.JWT.SigningKey),
	}, nil
}

type JWTHandler struct {
	SigningKey []byte
}

func (h *JWTHandler) Create(ctx *gin.Context) {
	var req types.CreateJWTReq
	if err := ctx.ShouldBind(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	claims := types.JWTClaims{
		CurrentUser:   req.CurrentUser,
		Organizations: req.Organizations,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			Issuer:    "OpenCSG",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(h.SigningKey)
	if err != nil {
		slog.Error("failed to generate JWT token: %v", err)
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, ss)
}
