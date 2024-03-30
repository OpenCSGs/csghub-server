package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/deploy/common"
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
	// schema := "http"
	// if IsWebSocketRequest(ctx.Request) {
	// 	fmt.Println("===get ws req", *ctx.Request)
	// 	schema = "ws"
	// }
	domainParts := strings.SplitN(host, ".", 2)
	spaceAppName := domainParts[0]
	// decode space id
	spaceID, err := common.ParseUniqueSpaceAppName(spaceAppName)
	if err != nil {
		slog.Info("proxy request has invalid space ID", slog.String("srv_name", spaceAppName), slog.Any("error", err))
		ctx.Status(http.StatusNotFound)
		return
	}

	// user verified by cookie
	username, exists := ctx.Get("currentUser")
	if !exists {
		httpbase.BadRequest(ctx, "user not found, please login first")
		return
	}

	allow, err := r.spaceComp.AllowCallApi(ctx, spaceID, username.(string))
	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}

	if allow {
		apiname := ctx.Param("api")
		// slog.Info("proxy space request", slog.String("namespace", namespace),
		// 	slog.String("name", name), slog.Any("username", username),
		// 	slog.String("api", apiname))
		var rp *proxy.ReverseProxy
		// fmt.Println(fmt.Sprintf("%s://%s.%s", schema, spaceAppName, r.SpaceRootDomain))
		// rp, _ = proxy.NewReverseProxy(fmt.Sprintf("%s://%s.%s", schema, spaceAppName, r.SpaceRootDomain))
		// fmt.Println(fmt.Sprintf("ws://%s.%s", spaceAppName, r.SpaceRootDomain))
		//
		//Special routing for graio app
		if apiname == "queue/join" {
			rp, _ = proxy.NewReverseProxy(fmt.Sprintf("ws://%s.%s", spaceAppName, r.SpaceRootDomain))
		} else {
			rp, _ = proxy.NewReverseProxy(fmt.Sprintf("http://%s.%s", spaceAppName, r.SpaceRootDomain))
		}
		rp.ServeHTTP(ctx.Writer, ctx.Request, apiname)
	} else {
		slog.Info("user not allowed to call sapce api", slog.String("srv_name", spaceAppName), slog.Any("user_name", username))
		ctx.Status(http.StatusForbidden)
	}
}

func IsWebSocketRequest(r *http.Request) bool {
	connHeader := ""
	connHeaders := r.Header["Connection"]
	if len(connHeaders) > 0 {
		connHeader = connHeaders[0]
	}

	upgradeWebsocket := false
	upgradeHeaders := r.Header["Upgrade"]
	if len(upgradeHeaders) > 0 {
		upgradeWebsocket = upgradeHeaders[0] == "websocket"
	}

	return strings.ToLower(connHeader) == "upgrade" && upgradeWebsocket
}
