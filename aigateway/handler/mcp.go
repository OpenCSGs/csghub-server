package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	gwcomp "opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type MCPProxyHandler interface {
	Resources(c *gin.Context)
	ProxyToApi(api string) gin.HandlerFunc
}

type MCPProxyHandlerImpl struct {
	spaceComp  component.SpaceComponent
	mcpResComp gwcomp.MCPResourceComponent
}

func NewMCPProxyHandler(config *config.Config) (MCPProxyHandler, error) {
	spaceComp, err := component.NewSpaceComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create space component,%w", err)
	}
	mcpResComp := gwcomp.NewMCPResourceComponent(config)
	return &MCPProxyHandlerImpl{
		spaceComp:  spaceComp,
		mcpResComp: mcpResComp,
	}, nil
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
		if !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "http://") {
			target = "https://" + target
		}
		rp, err := proxy.NewReverseProxy(target)
		if err != nil {
			slog.Error("fail to create mcp reverse proxy", slog.Any("err", err))
			httpbase.ServerError(ctx, err)
			return
		}
		rp.ServeHTTP(ctx.Writer, ctx.Request, api, "")
	}
}

// ListRecommendedMCPs      godoc
// @Security     ApiKey
// @Summary      List recommanded mcp servers
// @Description  Returns a list of recommended mcp servers
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(50)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=database.MCPResource,total=int} "OK"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/mcp/resources [get]
func (m *MCPProxyHandlerImpl) Resources(ctx *gin.Context) {
	mcpFilter := new(types.MCPFilter)
	mcpFilter.Username = httpbase.GetCurrentUser(ctx)
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Bad request format for pagination", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	mcpFilter.Per = per
	mcpFilter.Page = page
	mcps, total, err := m.mcpResComp.List(ctx.Request.Context(), mcpFilter)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to get mcp resources", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  mcps,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}
