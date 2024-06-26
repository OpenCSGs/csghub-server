package types

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CreateJWTReq struct {
	CurrentUser   string   `json:"current_user" binding:"required"`
	Organizations []string `json:"organizations"`
}

type CreateJWTResp struct {
	ExpireAt time.Time `json:"expire_at"`
	Token    string    `json:"token"`
}

type JWTClaims struct {
	CurrentUser   string   `json:"current_user"`
	Organizations []string `json:"organizations"`
	jwt.RegisteredClaims
}
