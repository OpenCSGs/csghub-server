package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

var ErrUnauthorized = errors.New("user not found in session, please access with jwt token first")

type RProxyHandler struct {
	clusterComp  component.ClusterComponent
	spaceComp    component.SpaceComponent
	repoComp     component.RepoComponent
	modSvcClient rpc.ModerationSvcClient
	cfg          *config.Config
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

	var modSvcClient rpc.ModerationSvcClient
	if config.SensitiveCheck.Enable {
		modSvcClient = rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", config.Moderation.Host, config.Moderation.Port))
	}

	clusterComp, err := component.NewClusterComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster component,%w", err)
	}

	return &RProxyHandler{
		clusterComp:  clusterComp,
		spaceComp:    spaceComp,
		repoComp:     repoComp,
		modSvcClient: modSvcClient,
		cfg:          config,
	}, nil
}

func (r *RProxyHandler) Proxy(ctx *gin.Context) {
	if ctx.Request.URL.Path == "/healthz" {
		ctx.Status(http.StatusOK)
		return
	}

	slog.Debug("http request proxy", slog.Any("request", ctx.Request.URL), slog.Any("header", ctx.Request.Header))
	appSvcName := r.getSvcName(ctx)

	deploy, err := r.repoComp.GetDeployBySvcName(ctx.Request.Context(), appSvcName)
	if err != nil {
		slog.Error("failed to get deploy in rproxy", slog.Any("error", err), slog.Any("appSrvName", appSvcName), slog.Any("request", ctx.Request.URL), slog.Any("header", ctx.Request.Header))
		httpbase.ServerError(ctx, fmt.Errorf("failed to get deploy, %w", err))
		return
	}
	username := httpbase.GetCurrentUser(ctx)
	allow, err := r.checkAccessPermission(ctx, deploy, username)

	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, ErrUnauthorized)
			return
		}

		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, fmt.Errorf("failed to check user permission,%w", err))
		return
	}

	if allow {
		apiname := ctx.Param("api")
		target, host, err := r.getSvcTargetAddress(ctx.Request.Context(), appSvcName, deploy)
		if err != nil {
			slog.Error("failed to get svc target address", slog.Any("error", err), slog.Any("appSvcName", appSvcName), slog.Any("request", ctx.Request.URL), slog.Any("header", ctx.Request.Header))
			httpbase.ServerError(ctx, fmt.Errorf("failed to forward request for svc %s, %w", appSvcName, err))
			return
		}
		slog.Info("proxy target", slog.Any("appSvcName", appSvcName), slog.Any("target", target),
			slog.Any("set-header-host", host), slog.Any("apiname", apiname))
		rp, _ := proxy.NewReverseProxy(target)
		if deploy.Type == types.InferenceType || deploy.Type == types.ServerlessType {
			// no need context path for inference
			contextPath := fmt.Sprintf("/%s/%s", "endpoint", appSvcName)
			apiname = strings.TrimPrefix(apiname, contextPath)
		}
		rp.ServeHTTP(ctx.Writer, ctx.Request, apiname, host)
	} else {
		slog.Warn("user not allowed to call endpoint api", slog.String("svc_name", appSvcName), slog.Any("user_name", username), slog.Any("deployID", deploy.ID))
		ctx.Status(http.StatusForbidden)
	}
}

func (r *RProxyHandler) checkAccessPermission(ctx *gin.Context, deploy *database.Deploy, username string) (bool, error) {
	var (
		allow bool
		err   error
		space *database.Space
	)
	if deploy.SpaceID > 0 {
		space, err = r.spaceComp.GetByID(ctx.Request.Context(), deploy.SpaceID)
		if err != nil {
			slog.Error("failed to get space by id", slog.Any("spaceID", deploy.SpaceID), slog.Any("error", err))
			return false, fmt.Errorf("failed to get space, %w", err)
		}
		// user must login to visit space except mcp server
		if space.Sdk != types.MCPSERVER.Name && httpbase.GetAuthType(ctx) != httpbase.AuthTypeJwt {
			slog.Error("invalid auth type in proxy", slog.Any("AuthType(ctx)", httpbase.GetAuthType(ctx)), slog.Any("URI", ctx.Request.RequestURI))
			return false, ErrUnauthorized
		}
		// check space
		allow, err = r.repoComp.AllowAccessByRepoID(ctx.Request.Context(), deploy.RepoID, username)
	} else {
		// check endpoint
		allow, err = r.repoComp.AllowAccessEndpoint(ctx.Request.Context(), username, deploy)
	}
	return allow, err
}

// get service name based on request
func (r *RProxyHandler) getSvcName(ctx *gin.Context) string {
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

func (r *RProxyHandler) getSvcTargetAddress(ctx context.Context, appSvcName string, deploy *database.Deploy) (string, string, error) {
	target := ""
	host := ""

	target = fmt.Sprintf("http://%s.%s", appSvcName, r.cfg.Space.InternalRootDomain)
	if len(deploy.Endpoint) > 0 {
		//support multi-cluster
		target = deploy.Endpoint
	}

	if len(deploy.ClusterID) < 1 {
		slog.Warn("cluster id of deploy svc is empty", slog.Any("svc", appSvcName))
		return target, host, nil
	}

	cluster, err := r.clusterComp.GetClusterByID(ctx, deploy.ClusterID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get cluster by id %s, error: %w", deploy.ClusterID, err)
	}

	if len(cluster.AppEndpoint) < 1 {
		slog.Warn("app endpoint of cluster is empty", slog.Any("clusterID", cluster.ClusterID))
		return target, host, nil
	}

	target = cluster.AppEndpoint
	if len(deploy.Endpoint) < 1 {
		return "", "", fmt.Errorf("endpoint of deploy %s is empty", appSvcName)
	}

	host, err = extractHostFromEndpoint(deploy.Endpoint)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract host from endpoint %s, error: %w", deploy.Endpoint, err)
	}

	return target, host, nil
}

func extractHostFromEndpoint(endpoint string) (string, error) {
	// http://u-neo888-test0922-2-lv.spaces.app.internal
	// extract host from url
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse endpoint url %s, error: %w", endpoint, err)
	}
	host := u.Hostname()
	if len(host) < 1 {
		return "", fmt.Errorf("extract host of endpoint %s is empty", endpoint)
	}
	return host, nil
}
