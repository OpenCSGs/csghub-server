package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewSpaceHandler(config *config.Config) (*SpaceHandler, error) {
	sc, err := component.NewSpaceComponent(config)
	if err != nil {
		return nil, err
	}
	rp, err := proxy.NewReverseProxy(config.Space.K8sEndpoint)
	if err != nil {
		// log error and continue
		slog.Error("failed to create space reverse proxy", slog.String("K8sEndpoint", config.Space.K8sEndpoint),
			slog.Any("error", err))
	}
	return &SpaceHandler{
		c:      sc,
		rproxy: rp,
	}, nil
}

type SpaceHandler struct {
	c      *component.SpaceComponent
	rproxy *proxy.ReverseProxy
}

// GetAllSpaces   godoc
// @Security     ApiKey
// @Summary      Get spaces visible to current user
// @Description  get spaces visible to current user
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Param        current_user query string false "current user"
// @Param        search query string false "search text"
// @Param        sort query string false "sort by"
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Space,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces [get]
func (h *SpaceHandler) Index(ctx *gin.Context) {
	username := ctx.Query("current_user")
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	search, sort := getFilterFromContext(ctx)
	if !slices.Contains[[]string](Sorts, sort) {
		msg := fmt.Sprintf("sort parameter must be one of %v", Sorts)
		slog.Error("Bad request format,", slog.String("error", msg))
		httpbase.BadRequest(ctx, msg)
		return
	}
	spaces, total, err := h.c.Index(ctx, username, search, sort, per, page)
	if err != nil {
		slog.Error("Failed to get spaces", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get public spaces succeed", slog.Int("count", total))
	respData := gin.H{
		"data":  spaces,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

func (h *SpaceHandler) Get(ctx *gin.Context) {
}

// CreateSpace   godoc
// @Security     ApiKey
// @Summary      Create a new space
// @Description  create a new space
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        body body types.CreateSpaceReq true "body"
// @Success      200  {object}  types.Response{data=types.Space} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces [post]
func (h *SpaceHandler) Create(ctx *gin.Context) {
	var req types.CreateSpaceReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	space, err := h.c.Create(ctx, req)
	if err != nil {
		slog.Error("Failed to create space", slog.Any("error", err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	slog.Info("Create space succeed", slog.String("space", space.Name))
	httpbase.OK(ctx, space)
}

func (h *SpaceHandler) Update(ctx *gin.Context) {
	type updateSpaceReq struct {
		Username string `json:"username" example:"creator_user_name"`
		License  string `json:"license" example:"MIT"`
		Private  bool   `json:"private"`
	}
}

func (h *SpaceHandler) Delete(ctx *gin.Context) {
}

// CreateSpace   godoc
// @Security     JWT token
// @Summary      Call space api
// @Description  call space api
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        api_name path string true "api_name"
// @Param        body body string false "body"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/api/{api_name} [post]
func (h *SpaceHandler) Proxy(ctx *gin.Context) {
	username, exists := ctx.Get("currentUser")
	if !exists {
		slog.Info("username not found in gin context")
		httpbase.BadRequest(ctx, "user not found, please login first")
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	allow, err := h.c.AllowCallApi(ctx, namespace, name, username.(string))
	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}

	if allow {
		apiname := ctx.Param("api_name")
		slog.Info("proxy space request", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", username),
			slog.String("api_name", apiname))
		h.rproxy.ServeHTTP(ctx.Writer, ctx.Request, apiname)
	} else {
		slog.Info("user not allowed to call sapce api", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", username))
	}
}
