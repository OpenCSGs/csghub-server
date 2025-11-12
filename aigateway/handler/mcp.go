package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type MCPProxyHandler interface {
	List(c *gin.Context)
	ProxyToApi(api string) gin.HandlerFunc
}

type MCPProxyHandlerImpl struct {
	spaceComp component.SpaceComponent
}

func NewMCPProxyHandler(config *config.Config) (MCPProxyHandler, error) {
	spaceComp, err := component.NewSpaceComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create space component,%w", err)
	}
	return &MCPProxyHandlerImpl{
		spaceComp: spaceComp,
	}, nil
}

// ListMCPs      godoc
// @Security     ApiKey
// @Summary      List available mcp servers
// @Description  Returns a list of available mcp servers
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.MCPService,total=int} "OK"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/mcp/servers [get]
func (m *MCPProxyHandlerImpl) List(ctx *gin.Context) {
	repoFilter := new(types.RepoFilter)
	repoFilter.Username = httpbase.GetCurrentUser(ctx)
	repoFilter.SpaceSDK = types.MCPSERVER.Name
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format for pagination", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	mcps, total, err := m.spaceComp.MCPIndex(ctx.Request.Context(), repoFilter, per, page)
	if err != nil {
		slog.Error("Failed to get mcp service", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  mcps,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

// proxy to mcp service
func (m *MCPProxyHandlerImpl) ProxyToApi(api string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		svcName := ctx.Param("servicename")
		slog.Debug("mcp proxy", slog.Any("svcName", svcName))
		mcpService, err := m.spaceComp.GetMCPServiceBySvcName(ctx.Request.Context(), svcName)
		if err != nil {
			if errors.Is(err, errorx.ErrNotFound) {
				httpbase.NotFoundError(ctx, fmt.Errorf("mcp service '%s' not found", svcName))
				return
			}
			slog.Error("fail to get mcp space", slog.Any("err", err), slog.String("svcName", svcName))
			httpbase.ServerError(ctx, err)
			return
		}

		if mcpService.Endpoint == "" {
			err := fmt.Errorf("mcp service '%s' is not running or has no endpoint", svcName)
			slog.Error("mcp space endpoint is empty", slog.Any("err", err), slog.String("svcName", svcName))
			httpbase.ServerError(ctx, err)
			return
		}
		api := ctx.Param("any")
		target := mcpService.Endpoint
		rp, err := proxy.NewReverseProxy(target)
		if err != nil {
			slog.Error("fail to create mcp reverse proxy", slog.Any("err", err))
			httpbase.ServerError(ctx, err)
			return
		}
		rp.ServeHTTP(ctx.Writer, ctx.Request, api, "")
	}
}
