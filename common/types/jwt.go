package types

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CreateJWTReq struct {
	UUID          string   `json:"uuid" binding:"required"`
	OldToken      string   `json:"old_token,omitempty"`
	Organizations []string `json:"-"`
}

type RefreshJWTReq struct {
	UUID     string `json:"uuid" binding:"required"`
	OldToken string `json:"old_token" binding:"required"`
}

type CreateJWTResp struct {
	ExpireAt time.Time `json:"expire_at"`
	Token    string    `json:"token"`
}

type JWTClaims struct {
	UUID        string `json:"uuid"`
	CurrentUser string `json:"current_user"`
	jwt.RegisteredClaims
}
