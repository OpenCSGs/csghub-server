package handler

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewSpaceResourceHandler(config *config.Config) (*SpaceResourceHandler, error) {
	src, err := component.NewSpaceResourceComponent(config)
	if err != nil {
		return nil, err
	}
	cluster, err := component.NewClusterComponent(config)
	if err != nil {
		return nil, err
	}
	return &SpaceResourceHandler{
		spaceResource: src,
		cluster:       cluster,
	}, nil
}

type SpaceResourceHandler struct {
	spaceResource component.SpaceResourceComponent
	cluster       component.ClusterComponent
}

// GetSpaceResources godoc
// @Security     ApiKey
// @Summary      Get space resources
// @Description  get space resources
// @Tags         SpaceReource
// @Accept       json
// @Produce      json
// @Param        per query int false "page size" default(50)
// @Param        page query int false "per page" default(1)
// @Param        cluster_id query string false "cluster_id"
// @Param 		 deploy_type query int false "deploy type(0-space,1-inference,2-finetune,3-serverless,4-evaluation)" Enums(0, 1, 2, 3, 4) default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.SpaceResource,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_resources [get]
func (h *SpaceResourceHandler) Index(ctx *gin.Context) {
	req := &types.SpaceResourceIndexReq{}
	if err := ctx.ShouldBindQuery(req); err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.CurrentUser = httpbase.GetCurrentUser(ctx)

	clusters := []string{}
	for _, c := range req.ClusterIDs {
		if c != "" {
			clusters = append(clusters, c)
		}
	}
	req.ClusterIDs = clusters

	if len(req.ClusterIDs) == 0 {
		clusters, err := h.cluster.Index(ctx.Request.Context())
		if err != nil {
			httpbase.ServerError(ctx, err)
			return
		}
		req.ClusterIDs = make([]string, len(clusters))
		for i := range clusters {
			req.ClusterIDs[i] = clusters[i].ClusterID
		}
	}

	if len(req.ClusterIDs) < 1 {
		httpbase.ServerError(ctx, errors.New("cluster is required"))
		return
	}

	spaceResources, total, err := h.spaceResource.Index(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get space resources", slog.Any("cluster_id", req.ClusterIDs), slog.Any("deploy_type", req.DeployType), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get space resources successfully")
	httpbase.OKWithTotal(ctx, spaceResources, total)
}

// CreateSpaceResource godoc
// @Security     ApiKey
// @Summary      Create space resource
// @Description  create space resource
// @Tags         SpaceReource
// @Accept       json
// @Produce      json
// @Param        body body types.CreateSpaceResourceReq true "body"
// @Success      200  {object}  types.ResponseWithTotal{data=types.SpaceResource,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_resources [post]
func (h *SpaceResourceHandler) Create(ctx *gin.Context) {
	var req types.CreateSpaceResourceReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	spaceResource, err := h.spaceResource.Create(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create space resources", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Create space resources successfully")
	httpbase.OK(ctx, spaceResource)
}

// UpdateSpaceResource godoc
// @Security     ApiKey
// @Summary      Update a exist space resource
// @Description  update a exist space resource
// @Tags         SpaceReource
// @Accept       json
// @Produce      json
// @Param        id path int true "id"
// @Param        body body types.UpdateSpaceResourceReq true "body"
// @Success      200  {object}  types.ResponseWithTotal{data=types.SpaceResource,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_resources/{id} [put]
func (h *SpaceResourceHandler) Update(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	var req *types.UpdateSpaceResourceReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.ID = id

	spaceResource, err := h.spaceResource.Update(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update space resource", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Update space resources successfully")
	httpbase.OK(ctx, spaceResource)
}

// DeleteSpaceResource godoc
// @Security     ApiKey
// @Summary      Delete a exist space resource
// @Description  delete a exist space resource
// @Tags         SpaceReource
// @Accept       json
// @Produce      json
// @Param        id path int true "id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_resources/{id} [delete]
func (h *SpaceResourceHandler) Delete(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = h.spaceResource.Delete(ctx.Request.Context(), id)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete space resource", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Delete space resource successfully")
	httpbase.OK(ctx, nil)
}

// ListHardwareTypes godoc
// @Security     ApiKey
// @Summary      List hardware types in a cluster
// @Description  list hardware types in a cluster
// @Tags         SpaceReource
// @Accept       json
// @Produce      json
// @Param        cluster_id query string false "cluster_id"
// @Success      200  {object}  types.Response{data=[]string} "OK"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /space_resources/hardware_types [get]

func (h *SpaceResourceHandler) ListHardwareTypes(ctx *gin.Context) {
	clusterId := ctx.Query("cluster_id")

	types, err := h.spaceResource.ListHardwareTypes(ctx.Request.Context(), clusterId)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to list hardware types", slog.String("cluster_id", clusterId), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("List hardware types successfully")
	httpbase.OK(ctx, types)
}
