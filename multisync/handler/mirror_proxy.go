package handler

import (
	"fmt"
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
	req.RepoPath = fmt.Sprintf("%s/%s", namespace, name)
	req.RepoType = repoType
	req.AccessToken = token
	err := r.mpComp.Serve(ctx, &req)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	path := strings.Replace(ctx.Request.URL.Path, fmt.Sprintf("%s/", repoType), fmt.Sprintf("%s_", repoType), 1)
	rp, _ := proxy.NewReverseProxy(r.gitServerURL)
	rp.ServeHTTP(ctx.Writer, ctx.Request, path)
}

func getMirrorTokenFromContext(ctx *gin.Context) string {
	return ctx.GetHeader(MirrorTokenHeaderKey)
}
