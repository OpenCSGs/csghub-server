package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

func NewClusterHandler(config *config.Config) (*ClusterHandler, error) {
	ncc, err := component.NewClusterComponent(config)
	if err != nil {
		return nil, err
	}
	return &ClusterHandler{
		c: ncc,
	}, nil
}

type ClusterHandler struct {
	c *component.ClusterComponent
}

// Getclusters   godoc
// @Security     ApiKey
// @Summary      Get cluster list
// @Description  Get cluster list
// @Tags         Cluster
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /cluster [get]
func (h *ClusterHandler) Index(ctx *gin.Context) {
	clusters, err := h.c.Index(ctx)
	if err != nil {
		slog.Error("Failed to get cluster list", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, clusters)
}
