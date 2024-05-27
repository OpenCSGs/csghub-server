package handler

import (
	"fmt"
	"log/slog"
	"net/http"
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
	repoComp        *component.RepoComponent
}

func NewRProxyHandler(config *config.Config) (*RProxyHandler, error) {
	spaceComp, err := component.NewSpaceComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create space component,%w", err)
	}
	repoComp, err := component.NewRepoComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo component,%w", err)
	}

	return &RProxyHandler{
		SpaceRootDomain: config.Space.InternalRootDomain,
		spaceComp:       spaceComp,
		repoComp:        repoComp,
	}, nil
}

func (r *RProxyHandler) Proxy(ctx *gin.Context) {
	slog.Debug("http request", slog.Any("request", ctx.Request.URL), slog.Any("header", ctx.Request.Header))
	host := ctx.Request.Host
	domainParts := strings.SplitN(host, ".", 2)
	appSrvName := domainParts[0]

	// user verified by cookie
	username, exists := ctx.Get("currentUser")
	if !exists {
		httpbase.BadRequest(ctx, "user not found, please login first")
		return
	}

	// spaceAppName := domainParts[0]
	// decode space id
	// spaceID, err := common.ParseUniqueSpaceAppName(spaceAppName)
	// if err != nil {
	// 	slog.Info("proxy request has invalid space ID", slog.String("srv_name", spaceAppName), slog.Any("error", err))
	// 	ctx.Status(http.StatusNotFound)
	// 	return
	// }

	// verify by space id get from url
	// allow, err := r.spaceComp.AllowCallApi(ctx, spaceID, username.(string))
	// if err != nil {
	// 	slog.Error("failed to check user permission", "error", err)
	// 	httpbase.ServerError(ctx, fmt.Errorf("failed to check user permission,%w", err))
	// 	return
	// }

	// verify by ksvc name
	allow, err := r.repoComp.AllowCallApi(ctx, appSrvName, username.(string))
	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, fmt.Errorf("failed to check user permission,%w", err))
		return
	}

	if allow {
		apiname := ctx.Param("api")
		target := fmt.Sprintf("http://%s.%s", appSrvName, r.SpaceRootDomain)
		rp, _ := proxy.NewReverseProxy(target)
		rp.ServeHTTP(ctx.Writer, ctx.Request, apiname)
	} else {
		slog.Info("user not allowed to call space api", slog.String("srv_name", appSrvName), slog.Any("user_name", username))
		ctx.Status(http.StatusForbidden)
	}
}
