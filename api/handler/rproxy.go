package handler

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
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
		SpaceRootDomain: config.Space.InternalRootDomain,
		spaceComp:       spaceComp,
	}, nil
}

func (r *RProxyHandler) Proxy(ctx *gin.Context) {
	slog.Debug("http request", slog.Any("request", ctx.Request.URL), slog.Any("header", ctx.Request.Header))
	host := ctx.Request.Host
	domainParts := strings.SplitN(host, ".", 2)
	spaceAppName := domainParts[0]
	// nameParts := strings.Split(spaceAppName, "-")
	// spaceIDStr := nameParts[len(nameParts)-1]
	// spaceID, err := strconv.ParseInt(spaceIDStr, 10, 64)
	// if err != nil {
	// 	slog.Info("proxy request has invalid space ID", slog.Any("error", err))
	// 	ctx.Status(http.StatusNotFound)
	// 	return
	// }
	//
	// username, exists := ctx.Get("currentUser")
	// if !exists {
	// 	slog.Info("username not found in gin context")
	// 	httpbase.BadRequest(ctx, "user not found, please login first")
	// 	return
	// }
	//
	// allow, err := r.spaceComp.AllowCallApi(ctx, namespace, name, username.(string))
	// if err != nil {
	// 	slog.Error("failed to check user permission", "error", err)
	// 	httpbase.ServerError(ctx, errors.New("failed to check user permission"))
	// 	return
	// }
	//
	allow := true
	if allow {
		apiname := ctx.Param("api")
		// slog.Info("proxy space request", slog.String("namespace", namespace),
		// 	slog.String("name", name), slog.Any("username", username),
		// 	slog.String("api", apiname))
		rp, _ := proxy.NewReverseProxy(fmt.Sprintf("http://%s.%s", spaceAppName, r.SpaceRootDomain))
		rp.ServeHTTP(ctx.Writer, ctx.Request, apiname)
	} else {
		// slog.Info("user not allowed to call sapce api", slog.Any("user_name", username))
	}
}
