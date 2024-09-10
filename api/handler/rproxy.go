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
	appSrvName := r.GetSrvName(ctx)

	deploy, err := r.repoComp.GetDeployBySvcName(ctx, appSrvName)
	if err != nil {
		slog.Error("failed to get deploy in rproxy", slog.Any("error", err), slog.Any("appSrvName", appSrvName))
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
		if deploy.Type == 1 || deploy.Type == 3 {
			//for infernece,no need context path
			contextPath := fmt.Sprintf("/%s/%s", "endpoint", appSrvName)
			apiname = strings.TrimPrefix(apiname, contextPath)
		}
		rp.ServeHTTP(ctx.Writer, ctx.Request, apiname)
	} else {
		slog.Warn("user not allowed to call endpoint api", slog.String("srv_name", appSrvName), slog.Any("user_name", username), slog.Any("deployID", deploy.ID))
		ctx.Status(http.StatusForbidden)
	}
}

// get service name based on request
func (r *RProxyHandler) GetSrvName(ctx *gin.Context) string {
	URI := ctx.Request.RequestURI
	host := ctx.Request.Host
	//check if request is from internal endpoint
	if strings.HasPrefix(URI, "/endpoint/") {
		//for case: http://127.0.0.1:8080/endpoint/dx1jpfny9hq8
		parts := strings.SplitN(ctx.Request.URL.Path, "/", 5)
		return parts[2]
	} else {
		// for case: https://dx1jpfny9hq8.cn-beijing.aliyun.space.opencsg.com
		domainParts := strings.SplitN(host, ".", 2)
		appSrvName := domainParts[0]
		return appSrvName
	}

}
