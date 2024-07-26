package handler

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/proxy"
)

type InternalServiceProxyHandler struct {
	rp *proxy.ReverseProxy
}

func NewInternalServiceProxyHandler(remoteEndpoint string) (*InternalServiceProxyHandler, error) {
	rp, err := proxy.NewReverseProxy(remoteEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create reverse proxy: %w", err)
	}
	return &InternalServiceProxyHandler{rp: rp}, nil
}

// Proxy send request to backend service, without change the request path
func (h *InternalServiceProxyHandler) Proxy(ctx *gin.Context) {
	// Log the request URL and header
	slog.Debug("http request", slog.Any("request", ctx.Request.URL), slog.Any("header", ctx.Request.Header))

	// Serve the request using the router
	h.rp.ServeHTTP(ctx.Writer, ctx.Request, "")
}

// ProxyToApi similar with Proxy, but change the request path
//
// the target request path can read params from the origin request path. This request will read 'username' from original request:
//
// apiGroup.PUT("/users/:username", userProxyHandler.ProxyToApi("/api/v1/user/%v", "username"))
func (h *InternalServiceProxyHandler) ProxyToApi(api string, originParams ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		slog.Info("proxy user request", slog.Any("request", ctx.Request.URL), slog.Any("header", ctx.Request.Header))
		finalApi := api
		if len(originParams) > 0 {
			var params []any
			for _, op := range originParams {
				params = append(params, ctx.Param(op))
			}
			finalApi = fmt.Sprintf(finalApi, params...)
		}
		h.rp.ServeHTTP(ctx.Writer, ctx.Request, finalApi)
	}
}
