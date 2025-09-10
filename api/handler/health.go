package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

type HealthHandler struct {
}

// Healthz godoc
// @Summary      Health check
// @Tags         Health
// @Router       /healthz [head]
func (h *HealthHandler) Healthz(ctx *gin.Context) {
	ctx.Status(http.StatusOK)
}
