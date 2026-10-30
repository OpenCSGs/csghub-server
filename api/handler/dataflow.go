package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	commonlog "opencsg.com/csghub-server/common/log"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type ForwardType string

const (
	ForwardTypeDataflow     ForwardType = "dataflow"
	ForwardTypeAcademy      ForwardType = "academy"
	ForwardTypeSupplydemand ForwardType = "supplydemand"
)

// DataflowProxyHandler forwards an authenticated identity to one configured external service.
type DataflowProxyHandler struct {
	forwardType ForwardType
	rp          proxy.ReverseProxy
	user        component.UserComponent
	usc         rpc.UserSvcClient
}

func NewDataflowProxyHandler(config *config.Config) (*DataflowProxyHandler, error) {
	return newThirdPartProxyHandler(config, ForwardTypeDataflow)
}

func NewAcademyProxyHandler(config *config.Config) (*DataflowProxyHandler, error) {
	return newThirdPartProxyHandler(config, ForwardTypeAcademy)
}

func NewSupplydemandProxyHandler(config *config.Config) (*DataflowProxyHandler, error) {
	return newThirdPartProxyHandler(config, ForwardTypeSupplydemand)
}

func newThirdPartProxyHandler(config *config.Config, forwardType ForwardType) (*DataflowProxyHandler, error) {
	uc, err := component.NewUserComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create user component: %w", err)
	}

	var remoteEndpoint string
	switch forwardType {
	case ForwardTypeDataflow:
		remoteEndpoint = fmt.Sprintf("%s:%d", config.Dataflow.Host, config.Dataflow.Port)
	case ForwardTypeAcademy:
		remoteEndpoint = fmt.Sprintf("%s:%d", config.Academy.Host, config.Academy.Port)
	case ForwardTypeSupplydemand:
		remoteEndpoint = fmt.Sprintf("%s:%d", config.Supplydemand.Host, config.Supplydemand.Port)
	default:
		return nil, fmt.Errorf("invalid forward type: %s", forwardType)
	}

	rp, err := proxy.NewReverseProxy(remoteEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s reverse proxy: %w", forwardType, err)
	}
	usc := rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))
	return &DataflowProxyHandler{rp: rp, user: uc, usc: usc, forwardType: forwardType}, nil
}

// Proxy preserves ordinary service paths and rebuilds the authenticated identity.
// Academy administration is available only through its explicit administrator routes.
func (h *DataflowProxyHandler) Proxy(ctx *gin.Context) {
	if h.forwardType == ForwardTypeAcademy {
		suffix := ctx.Param("any")
		clean := path.Clean("/" + strings.ReplaceAll(suffix, "\\", "/"))
		first, _, _ := strings.Cut(strings.TrimPrefix(clean, "/"), "/")
		if strings.EqualFold(first, "admin") {
			ctx.AbortWithStatus(http.StatusForbidden)
			return
		}
		// Reject ambiguous paths rather than letting the downstream router normalize them.
		if strings.ContainsAny(suffix, "%\\") || clean != strings.TrimSuffix(suffix, "/") && suffix != "/" {
			ctx.AbortWithStatus(http.StatusBadRequest)
			return
		}
		ctx.Request.URL.RawPath = ""
	}
	h.forward(ctx, "")
}

// forward applies the same identity contract to ordinary and mapped service paths.
func (h *DataflowProxyHandler) forward(ctx *gin.Context, api string) {
	if h.forwardType == ForwardTypeAcademy {
		// The academy must never receive identity headers asserted by a browser.
		for _, header := range []string{"User-Id", "User-Name", "User-Token", "User-Email", "X-Academy-User-Uuid"} {
			ctx.Request.Header.Del(header)
		}
	}
	// Client assertions are never used as an administrator identity.
	ctx.Request.Header.Set("isadmin", "false")
	currentUser := httpbase.GetCurrentUser(ctx)
	if len(currentUser) > 0 {
		u, err := h.user.GetUserByName(ctx.Request.Context(), currentUser)
		if err != nil || u == nil {
			if err == nil {
				err = errors.New("authenticated user not found")
			}
			httpbase.UnauthorizedError(ctx, err)
			ctx.Abort()
			return
		}
		token, err := h.usc.GetOrCreateFirstAvaiTokens(ctx.Request.Context(), currentUser, currentUser,
			string(types.AccessTokenAppGit), "dataflow")
		if err != nil {
			httpbase.ServerError(ctx, err)
			ctx.Abort()
			return
		}
		if len(token) == 0 {
			slog.ErrorContext(ctx.Request.Context(), "failed to get or create user first git access token",
				slog.Any("user", currentUser), slog.Any("error", err))
			httpbase.ServerError(ctx, errors.New("can not get user first available access token"))
			ctx.Abort()
			return
		}
		ctx.Request.Header.Set("User-Id", fmt.Sprintf("%d", u.ID))
		ctx.Request.Header.Set("User-Name", u.Username)
		ctx.Request.Header.Set("User-Token", token)
		ctx.Request.Header.Set("User-Email", u.Email)
		if h.forwardType == ForwardTypeAcademy {
			ctx.Request.Header.Set("X-Academy-User-Uuid", u.UUID)
		}
		ctx.Request.Header.Set("isadmin", fmt.Sprintf("%t", u.CanAdmin() || (h.forwardType == ForwardTypeAcademy && types.IsAdminAuthorized(ctx.Request.Context()))))
	}
	slog.DebugContext(ctx.Request.Context(), "proxy service request", slog.Any("service", h.forwardType), slog.Any("request", commonlog.SafeURL(ctx.Request.URL)), slog.Any("header", commonlog.SafeHeaders(ctx.Request.Header)))
	h.rp.ServeHTTP(ctx.Writer, ctx.Request, api, "")
}

// AcademyProxyToAPI forwards one fixed academy operation after route authorization.
// Identifiers cannot introduce path segments; query strings and bodies remain unchanged.
// The academy service must authenticate the original credentials independently.
func (h *DataflowProxyHandler) AcademyProxyToAPI(api string, originParams ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if h.forwardType != ForwardTypeAcademy || !types.IsAdminAuthorized(ctx.Request.Context()) || httpbase.GetCurrentUser(ctx) == "" {
			ctx.AbortWithStatus(http.StatusForbidden)
			return
		}
		params := make([]any, 0, len(originParams))
		for _, name := range originParams {
			value := ctx.Param(name)
			if !validAcademyIdentifier(value) {
				httpbase.BadRequest(ctx, "invalid academy identifier")
				return
			}
			params = append(params, value)
		}
		finalAPI := api
		if len(params) > 0 {
			finalAPI = fmt.Sprintf(api, params...)
		}
		// The fixed target replaces the incoming escaped path as well as its decoded path.
		ctx.Request.URL.RawPath = ""
		h.forward(ctx, finalAPI)
	}
}

// validAcademyIdentifier permits scalar resource IDs and UUIDs without path ambiguity.
func validAcademyIdentifier(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, ch := range value {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' || ch == '.' {
			continue
		}
		return false
	}
	return true
}

// ProxyToApi similar with Proxy, but change the request path
//
// the target request path can read params from the origin request path. This request will read 'id' from original request:
//
// apiGroup.PUT("/dataflow/job/get/:id", userProxyHandler.ProxyToApi("/api/v1/dataflow/job/get/%v", "id"))
func (h *DataflowProxyHandler) ProxyToApi(api string, originParams ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		slog.Info("proxy dataflow request", slog.Any("request", commonlog.SafeURL(ctx.Request.URL)), slog.Any("header", commonlog.SafeHeaders(ctx.Request.Header)))
		finalApi := api
		if len(originParams) > 0 {
			var params []any
			for _, op := range originParams {
				params = append(params, ctx.Param(op))
			}
			finalApi = fmt.Sprintf(finalApi, params...)
		}
		h.rp.ServeHTTP(ctx.Writer, ctx.Request, finalApi, "")
	}
}
