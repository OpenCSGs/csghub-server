package types

import "github.com/golang-jwt/jwt/v5"

type CreateJWTReq struct {
	CurrentUser   string   `json:"current_user" binding:"required"`
	Organizations []string `json:"organizations"`
}

type JWTClaims struct {
	CurrentUser   string   `json:"current_user"`
	Organizations []string `json:"organizations"`
	jwt.RegisteredClaims
}
