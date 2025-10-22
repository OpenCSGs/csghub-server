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
		// "agno": NewAgnoAdapter(config, agentComponent),
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
			httpbase.ServerError(ctx, fmt.Errorf("create reverse proxy: %w", err))
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
		slog.Debug("final api", "finalApi", finalApi)

		stream := ctx.Query("stream") == "true"
		if stream {
			ctx.Writer.Header().Set("Content-Type", "text/event-stream")
			ctx.Writer.Header().Set("Cache-Control", "no-cache")
			ctx.Writer.Header().Set("Connection", "keep-alive")
		}
		slog.Debug("stream", "stream", stream)

		w, err := adapter.PrepareResponseWriter(ctx, finalApi, stream)
		if err != nil {
			httpbase.ServerError(ctx, err)
			ctx.Abort()
			return
		}

		rp.ServeHTTP(w, ctx.Request, finalApi, "")
	}
}
