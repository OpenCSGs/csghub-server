package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	code "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
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
	c component.ClusterComponent
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
	clusters, err := h.c.Index(ctx.Request.Context())
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
	id := ctx.Param("id")
	cluster, err := h.c.GetClusterById(ctx.Request.Context(), id)
	if err != nil {
		slog.Error("Failed to get cluster", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, cluster)
}

// GetClusterUsage   godoc
// @Security     ApiKey
// @Summary      Get all cluster usage
// @Description  Get all cluster usage
// @Tags         Cluster
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /cluster/usage [get]
func (h *ClusterHandler) GetClusterUsage(ctx *gin.Context) {
	usages, err := h.c.GetClusterUsages(ctx.Request.Context())
	if err != nil {
		slog.Error("Failed to get cluster usage", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, usages)
}

// GetClusterDeploys  godoc
// @Security     ApiKey
// @Summary      Get cluster deploys
// @Description  Get cluster deploys
// @Tags         Cluster
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Param        per query int false "per" default(50)
// @Param        page query int false "page index" default(1)
// @Param        status query string false "status" default(all) Enums(all, running, stopped, deployfailed)
// @Param        search query string false "search" default("")
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /cluster/deploys [get]
func (h *ClusterHandler) GetDeploys(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format of page and per", slog.Any("error", err))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	var req types.DeployReq
	req.DeployTypes = []int{types.SpaceType, types.InferenceType, types.FinetuneType}
	req.Page = page
	req.PageSize = per
	status := ctx.Query("status")
	switch status {
	case "running":
		req.Status = []int{code.Running}
	case "stopped":
		req.Status = []int{code.Stopped}
	case "deployfailed":
		req.Status = []int{code.DeployFailed}
	}
	req.Query = ctx.Query("search")
	deploys, total, err := h.c.GetDeploys(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to get cluster deploys", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OKWithTotal(ctx, deploys, total)
}

func (h *ClusterHandler) Update(ctx *gin.Context) {
	var req types.ClusterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.ClusterID = ctx.Param("id")
	result, err := h.c.Update(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to update cluster info", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, result)
}
