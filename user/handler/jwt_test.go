package handler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

type failingJWTComponent struct{}

func (failingJWTComponent) GenerateLoginToken(context.Context, types.CreateJWTReq) (*types.JWTClaims, string, error) {
	return nil, "", errors.New("not implemented")
}

func (failingJWTComponent) RefreshToken(context.Context, types.RefreshJWTReq) (*types.JWTClaims, string, error) {
	return nil, "", errors.New("not implemented")
}

func (failingJWTComponent) GenerateToken(context.Context, types.CreateJWTReq) (*types.JWTClaims, string, error) {
	return nil, "", errors.New("not implemented")
}

func (failingJWTComponent) ParseToken(context.Context, string) (*types.User, error) {
	return nil, errors.New("invalid token")
}

func TestJWTHandlerVerifyDoesNotExposeToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "header.payload.signature"

	var logs bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	handler := &JWTHandler{c: failingJWTComponent{}}
	router := gin.New()
	router.GET("/jwt/:token", handler.Verify)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/jwt/"+secret, nil)

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.NotContains(t, response.Body.String(), secret)
	require.NotContains(t, logs.String(), secret)
	require.Contains(t, logs.String(), "failed to verify JWT token")
}
