package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AgentAdapter interface {
	Name() string
	GetHost(ctx context.Context) (string, error)
	PrepareResponseWriter(ctx *gin.Context, api string, stream bool) (http.ResponseWriter, error)
}
