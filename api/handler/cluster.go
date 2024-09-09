package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	clusters, err := h.c.Index(ctx)
	if err != nil {
		slog.Error("Failed to get cluster list", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, clusters)
}

// GetClusterById   godoc
// @Security     ApiKey
// @Summary      Get cluster by id
// @Description  Get cluster by id
// @Tags         Cluster
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /cluster/{id} [get]
func (h *ClusterHandler) GetClusterById(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	id := ctx.Param("id")
	cluster, err := h.c.GetClusterById(ctx, id)
	if err != nil {
		slog.Error("Failed to get cluster", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, cluster)
}

func (h *ClusterHandler) Update(ctx *gin.Context) {
	var req types.ClusterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.ClusterID = ctx.Param("id")
	result, err := h.c.Update(ctx, req)
	if err != nil {
		slog.Error("Failed to update cluster info", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, result)
}
