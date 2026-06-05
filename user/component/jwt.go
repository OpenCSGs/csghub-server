package component

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type jwtComponentImpl struct {
	SigningKey []byte
	ValidTime  time.Duration
	us         database.UserStore
}

type JwtComponent interface {
	// GenerateLoginToken generates a jwt token for a new login session.
	GenerateLoginToken(ctx context.Context, req types.CreateJWTReq) (claims *types.JWTClaims, signed string, err error)
	// RefreshToken generates a jwt token for an existing login session.
	RefreshToken(ctx context.Context, req types.RefreshJWTReq) (claims *types.JWTClaims, signed string, err error)
	// GenerateToken generate a jwt token, and return the token and signed string.
	// Deprecated: use GenerateLoginToken or RefreshToken instead.
	GenerateToken(ctx context.Context, req types.CreateJWTReq) (claims *types.JWTClaims, signed string, err error)
	ParseToken(ctx context.Context, token string) (user *types.User, err error)
}

func NewJwtComponent(signKey string, validHour int) JwtComponent {
	return &jwtComponentImpl{
		SigningKey: []byte(signKey),
		ValidTime:  time.Duration(validHour) * time.Hour,
		us:         database.NewUserStore(),
	}
}

func (c *jwtComponentImpl) GenerateLoginToken(ctx context.Context, req types.CreateJWTReq) (claims *types.JWTClaims, signed string, err error) {
	u, err := c.us.FindByUUID(ctx, req.UUID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find user by uuid '%s',error: %w", req.UUID, err)
	}

	claims = c.loginClaims(u)
	signed, err = c.signClaims(claims)
	if err != nil {
		return nil, "", fmt.Errorf("generate jwt token failed: %w", err)
	}

	return claims, signed, nil
}

func (c *jwtComponentImpl) RefreshToken(ctx context.Context, req types.RefreshJWTReq) (claims *types.JWTClaims, signed string, err error) {
	u, err := c.us.FindByUUID(ctx, req.UUID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find user by uuid '%s',error: %w", req.UUID, err)
	}

	claims, err = c.refreshClaims(req, u)
	if err != nil {
		return nil, "", err
	}
	signed, err = c.signClaims(claims)
	if err != nil {
		return nil, "", fmt.Errorf("refresh jwt token failed: %w", err)
	}
	return claims, signed, nil
}

// GenerateToken generate a jwt token, and return the token and signed string.
func (c *jwtComponentImpl) GenerateToken(ctx context.Context, req types.CreateJWTReq) (claims *types.JWTClaims, signed string, err error) {
	if req.OldToken != "" {
		return c.RefreshToken(ctx, types.RefreshJWTReq{UUID: req.UUID, OldToken: req.OldToken})
	}
	u, err := c.us.FindByUUID(ctx, req.UUID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find user by uuid '%s',error: %w", req.UUID, err)
	}
	claims, err = c.createClaims(u)
	if err != nil {
		return nil, "", err
	}
	signed, err = c.signClaims(claims)
	if err != nil {
		return nil, "", fmt.Errorf("generate jwt token failed: %w", err)
	}
	return claims, signed, nil
}

func (c *jwtComponentImpl) ParseToken(ctx context.Context, token string) (user *types.User, err error) {
	claims, err := c.parseClaims(token)
	if err != nil {
		return nil, fmt.Errorf("parse jwt token failed: %w", err)
	}

	dbu, err := c.us.FindByUsername(ctx, claims.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to find user by name '%s', %w", claims.CurrentUser, err)
	}

	// create new user object
	u := &types.User{
		UUID:              dbu.UUID,
		Username:          dbu.Username,
		Email:             dbu.Email,
		Roles:             dbu.Roles(),
		CanChangeUserName: dbu.CanChangeUserName,
	}
	return u, nil
}

func (c *jwtComponentImpl) buildClaims(u *database.User, expireAt time.Time) *types.JWTClaims {
	return &types.JWTClaims{
		UUID:        u.UUID,
		CurrentUser: u.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expireAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "OpenCSG",
		},
	}
}

func (c *jwtComponentImpl) parseClaims(token string) (*types.JWTClaims, error) {
	claims := &types.JWTClaims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(token *jwt.Token) (interface{}, error) {
			return c.SigningKey, nil
		},
		jwt.WithIssuer("OpenCSG"),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %w", errorx.ErrInvalidJWT, err)
		}
		return nil, err
	}
	return claims, nil
}

func (c *jwtComponentImpl) signClaims(claims *types.JWTClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(c.SigningKey)
}
