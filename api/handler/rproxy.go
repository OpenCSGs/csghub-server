package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

type RProxyHandler struct {
	SpaceRootDomain string
	spaceComp       *component.SpaceComponent
}

func NewRProxyHandler(config *config.Config) (*RProxyHandler, error) {
	spaceComp, err := component.NewSpaceComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create space component,%w", err)
	}
	return &RProxyHandler{
		SpaceRootDomain: config.Space.RootDomain,
		spaceComp:       spaceComp,
	}, nil
}

func (r *RProxyHandler) Proxy(ctx *gin.Context) {
	slog.Debug("http request", slog.Any("request", ctx.Request.URL), slog.Any("header", ctx.Request.Header))
	host := ctx.Request.Host
	domainParts := strings.SplitN(host, ".", 2)
	spaceSrvName := domainParts[0]
	names := strings.SplitN(spaceSrvName, "-", 2)
	namespace, name := names[0], names[1]

	username, exists := ctx.Get("currentUser")
	if !exists {
		slog.Info("username not found in gin context")
		httpbase.BadRequest(ctx, "user not found, please login first")
		return
	}

	allow, err := r.spaceComp.AllowCallApi(ctx, namespace, name, username.(string))
	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}

	if allow {
		apiname := ctx.Param("api")
		slog.Info("proxy space request", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", username),
			slog.String("api", apiname))
		rp, _ := proxy.NewReverseProxy(fmt.Sprintf("http://%s.%s", spaceSrvName, r.SpaceRootDomain))
		rp.ServeHTTP(ctx.Writer, ctx.Request, apiname)
	} else {
		slog.Info("user not allowed to call sapce api", slog.Any("user_name", username))
	}
}
