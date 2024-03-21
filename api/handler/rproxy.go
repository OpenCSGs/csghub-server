package handler

import (
	"fmt"
	"log/slog"
	"net/http/httputil"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/config"
)

type RProxyHandler struct {
	SpaceRootDomain string
}

func NewRProxyHandler(config *config.Config) (*RProxyHandler, error) {
	return &RProxyHandler{
		SpaceRootDomain: config.Space.RootDomain,
	}, nil
}

func (r *RProxyHandler) Proxy(ctx *gin.Context) {
	slog.Debug("http request", slog.Any("request", ctx.Request.URL), slog.Any("header", ctx.Request.Header))
	data, _ := httputil.DumpRequest(ctx.Request, true)
	fmt.Println(string(data))
	allow := true
	// username, exists := ctx.Get("currentUser")
	// if !exists {
	// 	slog.Info("username not found in gin context")
	// 	httpbase.BadRequest(ctx, "user not found, please login first")
	// 	return
	// }
	// namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	// if err != nil {
	// 	slog.Error("failed to get namespace from context", "error", err)
	// 	httpbase.BadRequest(ctx, err.Error())
	// 	return
	// }
	// allow, err := h.c.AllowCallApi(ctx, namespace, name, username.(string))
	// if err != nil {
	// 	slog.Error("failed to check user permission", "error", err)
	// 	httpbase.ServerError(ctx, errors.New("failed to check user permission"))
	// 	return
	// }
	//
	if allow {
		apiname := ctx.Param("api")
		// slog.Info("proxy space request", slog.String("namespace", namespace),
		// 	slog.String("name", name), slog.Any("username", username),
		// 	slog.String("api_name", apiname))
		host := ctx.Request.Host
		if len(host) == 0 {
			host = ctx.GetHeader("Host")
		}
		fmt.Println("host", host, len(host))
		domainParts := strings.SplitN(host, ".", 2)
		rp, _ := proxy.NewReverseProxy(fmt.Sprintf("http://%s.%s", domainParts[0], r.SpaceRootDomain))
		rp.ServeHTTP(ctx.Writer, ctx.Request, apiname)
	} else {
		slog.Info("user not allowed to call sapce api")
	}
}
