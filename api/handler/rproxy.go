package handler

import (
	"errors"
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

	deploy, err := r.repoComp.GetDeployBySvcName(ctx, appSrvName)
	if err != nil {
		slog.Error("failed to get deploy", slog.Any("error", err), slog.Any("appSrvName", appSrvName))
		httpbase.ServerError(ctx, fmt.Errorf("failed to get deploy, %w", err))
		return
	}

	username := httpbase.GetCurrentUser(ctx)
	allow := false
	err = nil
	if deploy.SpaceID > 0 {
		// user must login to visit space
		if httpbase.GetAuthType(ctx) != httpbase.AuthTypeJwt {
			httpbase.UnauthorizedError(ctx, errors.New("user not found in session, please access with jwt token first"))
			return
		}

		// check space
		allow, err = r.repoComp.AllowAccessByRepoID(ctx, deploy.RepoID, username)
	} else if deploy.ModelID > 0 {
		// check model inference
		allow, err = r.repoComp.AllowAccessEndpoint(ctx, username, deploy)
	}

	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, fmt.Errorf("failed to check user permission,%w", err))
		return
	}

	if allow {
		apiname := ctx.Param("api")
		target := fmt.Sprintf("http://%s.%s", appSrvName, r.SpaceRootDomain)
		if deploy.Endpoint != "" {
			//support multi-cluster
			target = deploy.Endpoint
		}
		rp, _ := proxy.NewReverseProxy(target)
		rp.ServeHTTP(ctx.Writer, ctx.Request, apiname)
	} else {
		slog.Warn("user not allowed to call endpoint api", slog.String("srv_name", appSrvName), slog.Any("user_name", username), slog.Any("deployID", deploy.ID))
		ctx.Status(http.StatusForbidden)
	}
}
