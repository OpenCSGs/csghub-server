package handler

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component"
)

const MirrorTokenHeaderKey = "X-OPENCSG-Sync-Token"

type MirrorProxyHandler struct {
	gitServerURL string
	repoComp     *component.RepoComponent
}

func NewMirrorProxyHandler(config *config.Config) (*MirrorProxyHandler, error) {
	repoComp, err := component.NewRepoComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo component,%w", err)
	}

	return &MirrorProxyHandler{
		repoComp:     repoComp,
		gitServerURL: config.GitServer.URL,
	}, nil
}

func (r *MirrorProxyHandler) Serve(ctx *gin.Context) {
	// slog.Debug("http request", slog.Any("request", ctx.Request.URL), slog.Any("header", ctx.Request.Header))
	// host := ctx.Request.Host
	// domainParts := strings.SplitN(host, ".", 2)
	// appSrvName := domainParts[0]

	// // user verified by cookie
	// username, exists := ctx.Get("currentUser")
	// if !exists {
	// 	httpbase.BadRequest(ctx, "user not found, please login first")
	// 	return
	// }

	// deploy, err := r.repoComp.GetDeployBySvcName(ctx, appSrvName)
	// if err != nil {
	// 	slog.Error("failed to get deploy", slog.Any("error", err), slog.Any("appSrvName", appSrvName))
	// 	httpbase.ServerError(ctx, fmt.Errorf("failed to get deploy, %w", err))
	// 	return
	// }

	// allow := false
	// err = nil
	// // verify by repo_id, user name
	// if deploy.SpaceID > 0 {
	// 	// check space
	// 	allow, err = r.repoComp.AllowAccessByRepoID(ctx, deploy.RepoID, username.(string))
	// } else if deploy.ModelID > 0 {
	// 	// check model inference
	// 	allow, err = r.repoComp.AllowAccessEndpoint(ctx, username.(string), deploy)
	// }

	// if err != nil {
	// 	slog.Error("failed to check user permission", "error", err)
	// 	httpbase.ServerError(ctx, fmt.Errorf("failed to check user permission,%w", err))
	// 	return
	// }

	// Accounting
	token := getMirrorTokenFromContext(ctx)
	fmt.Println(token)
	repoType := ctx.Param("repo_type")
	path := strings.Replace(ctx.Request.URL.Path, fmt.Sprintf("%s/", repoType), fmt.Sprintf("%s_", repoType), 1)
	rp, _ := proxy.NewReverseProxy(r.gitServerURL)
	rp.ServeHTTP(ctx.Writer, ctx.Request, path)
}

func getMirrorTokenFromContext(ctx *gin.Context) string {
	return ctx.GetHeader(MirrorTokenHeaderKey)
}
