package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type MCPServerHandler struct {
	mcpComp   component.MCPServerComponent
	sensitive component.SensitiveComponent
}

func NewMCPServerHandler(config *config.Config) (*MCPServerHandler, error) {
	mc, err := component.NewMCPServerComponent(config)
	if err != nil {
		return nil, err
	}
	sc, err := component.NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	return &MCPServerHandler{
		mcpComp:   mc,
		sensitive: sc,
	}, nil
}

// CreateMCPServer    godoc
// @Security     ApiKey
// @Summary      Create a new mcp server
// @Description  create a new mcp server
// @Tags         MCP
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user, the owner"
// @Param        body body types.CreateMCPServerReq true "body"
// @Success      200  {object}  types.Response{data=types.MCPServer} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mcps [post]
func (h *MCPServerHandler) Create(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	var req *types.CreateMCPServerReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format for create mcp server", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to check sensitive for mcp create request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	req.Username = currentUser

	mcpServer, err := h.mcpComp.Create(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, component.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("failed to create mcp server", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, mcpServer)
}

// DeleteMCPServer    godoc
// @Security     ApiKey
// @Summary      Delete a exists mcp server
// @Description  delete a exists mcp server
// @Tags         MCP
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user, the owner"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mcps/{namespace}/{name} [delete]
func (h *MCPServerHandler) Delete(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format for remove mcp server", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req := &types.UpdateMCPServerReq{
		UpdateRepoReq: types.UpdateRepoReq{
			Namespace: namespace,
			Name:      name,
			Username:  currentUser,
		},
	}

	err = h.mcpComp.Delete(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, component.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("failed to delete mcp server", slog.Any("error", err), slog.Any("namespace", namespace), slog.Any("name", name))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// UpdateMCPServer   godoc
// @Security     ApiKey
// @Summary      Update a exists mcp server
// @Description  update a exists mcp server
// @Tags         MCP
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user, the owner"
// @Param        body body types.UpdateMCPServerReq true "body"
// @Success      200  {object}  types.Response{data=types.MCPServer} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mcps/{namespace}/{name} [put]
func (h *MCPServerHandler) Update(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad mcp server request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var req *types.UpdateMCPServerReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request body format for update mcp server", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.Username = currentUser
	req.Namespace = namespace
	req.Name = name

	_, err = h.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to check sensitive request for update mcp server", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	res, err := h.mcpComp.Update(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, component.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("failed to update mcp server", slog.Any("error", err), slog.Any("ns", namespace), slog.Any("name", name))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, res)
}

// GetMCPServer  godoc
// @Security     ApiKey
// @Summary      Get mcp server detail
// @Description  Get mcp server detail
// @Tags         MCP
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Success      200  {object}  types.Response{data=types.MCPServer} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mcps/{namespace}/{name} [get]
func (h *MCPServerHandler) Show(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)

	qNeedOpWeight := ctx.Query("need_op_weight")
	needOpWeight, err := strconv.ParseBool(qNeedOpWeight)
	if err != nil {
		needOpWeight = false
	}

	detail, err := h.mcpComp.Show(ctx.Request.Context(), namespace, name, currentUser, needOpWeight)
	if err != nil {
		if errors.Is(err, component.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.Error("failed to get mcp server", slog.Any("error", err), slog.Any("ns", namespace), slog.Any("name", name))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, detail)
}

// GetVisiableMCPServers godoc
// @Security     ApiKey
// @Summary      Get Visiable mcp servers for current user
// @Description  Get visiable mcp servers for current user
// @Tags         MCP
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user"
// @Param        tag_category query string false "filter by tag category"
// @Param        tag_name query string false "filter by tag name"
// @Param        tag_group query string false "filter by tag group"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Param        search query string false "search text"
// @Param        sort query string false "sort by"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.MCPServer,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mcps [get]
func (h *MCPServerHandler) Index(ctx *gin.Context) {
	filter := new(types.RepoFilter)
	filter.Tags = parseTagReqs(ctx)
	filter.Username = httpbase.GetCurrentUser(ctx)
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format for mcp list", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filter = getFilterFromContext(ctx, filter)
	if !slices.Contains(Sorts, filter.Sort) {
		msg := fmt.Sprintf("sort parameter must be one of %v", Sorts)
		slog.Error("check list mcp server filter", slog.String("msg", msg))
		ctx.JSON(http.StatusBadRequest, gin.H{"message": msg})
		return
	}

	qNeedOpWeight := ctx.Query("need_op_weight")
	needOpWeight, err := strconv.ParseBool(qNeedOpWeight)
	if err != nil {
		needOpWeight = false
	}

	mcps, total, err := h.mcpComp.Index(ctx.Request.Context(), filter, per, page, needOpWeight)
	if err != nil {
		slog.Error("failed to get mcp servers", slog.Any("error", err), slog.Any("filter", filter))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  mcps,
		"total": total,
	}
	httpbase.OK(ctx, respData)
}

// GetVisiableMCPServersTools godoc
// @Security     ApiKey
// @Summary      Get Visiable mcp servers tools for current user
// @Description  Get visiable mcp servers tools for current user
// @Tags         MCP
// @Accept       json
// @Produce      json
// @Param        search query string false "search text"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.MCPServerProperties,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mcps/tools [get]
func (h *MCPServerHandler) Properties(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	kind := types.MCPPropTool
	search := ctx.Query("search")
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format for mcp property list", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req := &types.MCPPropertyFilter{
		CurrentUser: currentUser,
		Search:      search,
		Kind:        kind,
		Per:         per,
		Page:        page,
	}

	properties, total, err := h.mcpComp.Properties(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to get mcp tool properties", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	respData := gin.H{
		"data":  properties,
		"total": total,
	}
	httpbase.OK(ctx, respData)
}
