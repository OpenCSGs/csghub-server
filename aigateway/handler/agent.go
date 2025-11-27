package handler

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

type AgentProxyHandler interface {
	ProxyToApi(api string, originParams ...string) gin.HandlerFunc
}

type AgentProxyHandlerImpl struct {
	config   *config.Config
	adapters map[string]AgentAdapter
}

var _ AgentProxyHandler = (*AgentProxyHandlerImpl)(nil)

func NewAgentProxyHandler(config *config.Config) (AgentProxyHandler, error) {
	agentComponent, err := component.NewAgentComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent component: %w", err)
	}
	adapters := map[string]AgentAdapter{
		"langflow": NewLangflowAdapter(config, agentComponent),
		"code":     NewCodeAdapter(config, agentComponent),
	}
	return &AgentProxyHandlerImpl{
		config:   config,
		adapters: adapters,
	}, nil
}

func (h *AgentProxyHandlerImpl) ProxyToApi(api string, originParams ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		slog.Debug("agent proxy request", "request", ctx.Request.URL.String(), "header", ctx.Request.Header)

		agentType := ctx.Param("type")
		adapter, ok := h.adapters[agentType]
		if !ok {
			httpbase.BadRequest(ctx, fmt.Sprintf("unsupported agent type: %s", agentType))
			ctx.Abort()
			return
		}

		host, err := adapter.GetHost(ctx)
		if err != nil {
			httpbase.ServerError(ctx, err)
			ctx.Abort()
			return
		}
		rp, err := proxy.NewReverseProxy(host)
		if err != nil {
			slog.Error("failed to create reverse proxy", "agent_type", agentType, "host", host, "error", err)
			httpbase.ServerError(ctx, err)
			ctx.Abort()
			return
		}

		finalApi := api
		if len(originParams) > 0 {
			var params []any
			for _, op := range originParams {
				params = append(params, ctx.Param(op))
			}
			finalApi = fmt.Sprintf(finalApi, params...)
		}
		slog.Debug("agent proxy request", "agent_type", agentType, "api", finalApi, "request_headers", ctx.Request.Header)

		if err := adapter.PrepareProxyContext(ctx, finalApi); err != nil {
			slog.Error("failed to prepare proxy context", "agent_type", agentType, "api", finalApi, "error", err)
			httpbase.ServerError(ctx, err)
			ctx.Abort()
			return
		}

		rp.ServeHTTP(ctx.Writer, ctx.Request, finalApi, "")
	}
}
