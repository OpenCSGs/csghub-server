package handler

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/multisync/component"
	"opencsg.com/csghub-server/multisync/types"
)

const MirrorTokenHeaderKey = "X-OPENCSG-Sync-Token"

type MirrorProxyHandler struct {
	gitServerURL string
	mpComp       *component.MirrorProxyComponent
}

func NewMirrorProxyHandler(config *config.Config) (*MirrorProxyHandler, error) {
	mpComp, err := component.NewMirrorProxyComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo component,%w", err)
	}

	return &MirrorProxyHandler{
		mpComp:       mpComp,
		gitServerURL: config.GitServer.URL,
	}, nil
}

func (r *MirrorProxyHandler) Serve(ctx *gin.Context) {
	var req types.GetSyncQuotaStatementReq
	token := getMirrorTokenFromContext(ctx)
	repoType := ctx.Param("repo_type")
	namespace := ctx.Param("namespace")
	name := ctx.Param("name")
	name, _ = strings.CutSuffix(name, ".git")
	req.RepoPath = fmt.Sprintf("%s/%s", namespace, name)
	req.RepoType = strings.TrimSuffix(repoType, "s")
	req.AccessToken = token

	if strings.HasSuffix(ctx.Request.URL.Path, "git-upload-pack") {
		err := r.mpComp.Serve(ctx, &req)
		if err != nil {
			slog.Error("failed to serve git upload pack request:", slog.Any("err", err))
			httpbase.BadRequest(ctx, err.Error())
			return
		}
	}

	rp, _ := proxy.NewReverseProxy(r.gitServerURL)
	rp.ServeHTTP(ctx.Writer, ctx.Request, ctx.Request.URL.Path, "")
}

func (r *MirrorProxyHandler) ServeLFS(ctx *gin.Context) {
	rp, _ := proxy.NewReverseProxy(r.gitServerURL)
	rp.ServeHTTP(ctx.Writer, ctx.Request, ctx.Request.URL.Path, "")
}

func getMirrorTokenFromContext(ctx *gin.Context) string {
	csgHeaderToken := ctx.GetHeader(MirrorTokenHeaderKey)
	if csgHeaderToken != "" {
		return csgHeaderToken
	}
	authToken := ctx.GetHeader("Authorization")
	authToken = strings.TrimPrefix(authToken, "Bearer ")
	authToken = strings.TrimPrefix(authToken, MirrorTokenHeaderKey)
	return authToken
}
