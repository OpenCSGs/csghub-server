package handler

import (
	"context"

	"github.com/gin-gonic/gin"
)

type AgentAdapter interface {
	Name() string
	GetHost(ctx context.Context) (string, error)
	PrepareProxyContext(ctx *gin.Context, api string) error
}
