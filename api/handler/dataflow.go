package handler

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type DataflowProxyHandler struct {
	rp   proxy.ReverseProxy
	user component.UserComponent
	usc  rpc.UserSvcClient
}

func NewDataflowProxyHandler(config *config.Config) (*DataflowProxyHandler, error) {
	uc, err := component.NewUserComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create user component: %w", err)
	}
	remoteEndpoint := fmt.Sprintf("%s:%d", config.Dataflow.Host, config.Dataflow.Port)
	rp, err := proxy.NewReverseProxy(remoteEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create dataflow reverse proxy: %w", err)
	}
	usc := rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))
	return &DataflowProxyHandler{rp: rp, user: uc, usc: usc}, nil
}

// Proxy send request to backend service, without change the request path
func (h *DataflowProxyHandler) Proxy(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	u, err := h.user.GetUserByName(ctx.Request.Context(), currentUser)
	if err != nil {
		httpbase.UnauthorizedError(ctx, err)
		ctx.Abort()
		return
	}
	token, err := h.usc.GetOrCreateFirstAvaiTokens(ctx.Request.Context(), currentUser, currentUser, string(types.AccessTokenAppGit), "dataflow")
	if err != nil {

		httpbase.ServerError(ctx, err)
		ctx.Abort()
		return
	}
	if len(token) == 0 {
		slog.Error("fail to get or create user first git access token", slog.Any("user", currentUser), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New("can not get user first available access token"))
		ctx.Abort()
		return
	}
	ctx.Request.Header.Set("User-Id", fmt.Sprintf("%d", u.ID))
	ctx.Request.Header.Set("User-Name", u.Username)
	ctx.Request.Header.Set("User-Token", token)
	ctx.Request.Header.Set("User-Email", u.Email)
	ctx.Request.Header.Set("isadmin", fmt.Sprintf("%t", u.CanAdmin()))
	// Log the request URL and header
	slog.Debug("http request", slog.Any("request", ctx.Request.URL), slog.Any("header", ctx.Request.Header))
	// Serve the request using the router
	h.rp.ServeHTTP(ctx.Writer, ctx.Request, "", "")
}

// ProxyToApi similar with Proxy, but change the request path
//
// the target request path can read params from the origin request path. This request will read 'id' from original request:
//
// apiGroup.PUT("/dataflow/job/get/:id", userProxyHandler.ProxyToApi("/api/v1/dataflow/job/get/%v", "id"))
func (h *DataflowProxyHandler) ProxyToApi(api string, originParams ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		slog.Info("proxy dataflow request", slog.Any("request", ctx.Request.URL), slog.Any("header", ctx.Request.Header))
		finalApi := api
		if len(originParams) > 0 {
			var params []any
			for _, op := range originParams {
				params = append(params, ctx.Param(op))
			}
			finalApi = fmt.Sprintf(finalApi, params...)
		}
		h.rp.ServeHTTP(ctx.Writer, ctx.Request, finalApi, "")
	}
}
