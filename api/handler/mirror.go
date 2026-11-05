package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

// create new MirrorHandler
func NewMirrorHandler(config *config.Config) (*MirrorHandler, error) {
	mc, err := component.NewMirrorComponent(config)
	if err != nil {
		return nil, err
	}
	handler := &MirrorHandler{
		mirror: mc,
	}
	if err := configureBatchMirrorHandler(handler, mc); err != nil {
		return nil, err
	}
	return handler, nil
}

type batchMirrorComponent interface {
	BatchCreate(ctx context.Context, req types.BatchCreateMirrorReq) error
}

type MirrorHandler struct {
	mirror      component.MirrorComponent
	batchMirror batchMirrorComponent
}

// CreateMirrorRepo godoc
// @Security     ApiKey
// @Summary      Create mirror repo
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        body body types.CreateMirrorRepoReq true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      404  {object}  types.APINotFound "Not found"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/repo [post]
func (h *MirrorHandler) CreateMirrorRepo(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	var req types.CreateMirrorRepoReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		// A decoded URL may be invalid even when its derived source fields are missing.
		// Prioritize that error only after successful JSON decoding; preserve transport errors.
		var fieldErrors validator.ValidationErrors
		if errors.As(err, &fieldErrors) && req.SourceGitCloneUrl != "" {
			if sourceErr := component.ValidateMirrorSourceURL(req.SourceGitCloneUrl); sourceErr != nil {
				httpbase.BadRequestWithExt(ctx, sourceErr)
				return
			}
		}
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if strings.TrimSpace(req.ForkNamespace) == "" || strings.TrimSpace(req.ForkName) == "" {
		httpbase.BadRequest(ctx, "fork_namespace and fork_name are required")
		return
	}

	req.CurrentUser = currentUser
	// Mirror API requests default to public while preserving explicit private settings.
	if req.Private == nil {
		req.Private = new(false)
	}
	req.AllowAutoCreateOrganization = req.AutoCreateOrganization == nil || *req.AutoCreateOrganization
	m, err := h.mirror.CreateMirrorRepo(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to create mirror repo", slog.Any("error", err))
		if errors.Is(err, errorx.ErrMirrorSourceRepoAuthInvalid) ||
			errors.Is(err, errorx.ErrMirrorSourceURLInvalid) ||
			errors.Is(err, errorx.ErrBadRequest) {
			httpbase.BadRequestWithExt(ctx, err)
			return
		}
		if errors.Is(err, errorx.ErrRepoNotFound) {
			httpbase.NotFoundError(ctx, err)
			return
		}
		httpbase.ServerError(ctx, err)
		return
	}
	if m != nil {
		slog.Debug("create mirror repo", slog.Any("mirror", m.Repository), slog.Any("req", req))
	}

	httpbase.OK(ctx, nil)
}

// GetMirrorRepos godoc
// @Security     ApiKey
// @Summary      Get repositories in the multi-source synchronization queue
// @Description  Requires asset/sync administrator permission.
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(20)
// @Param        page query int false "page" default(1)
// @Success      200  {object}  types.Response{data=[]types.MirrorRepo,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Unauthorized"
// @Failure      403  {object}  types.APIForbidden "Forbidden"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/repos [get]
func (h *MirrorHandler) Repos(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	repos, total, err := h.mirror.Repos(ctx.Request.Context(), per, page)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get mirror repos", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  repos,
		"total": total,
	}

	httpbase.OK(ctx, respData)
}

// Index godoc
// @Security     ApiKey
// @Summary      Get mirrors, used for admin
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        per query int false "per" default(20)
// @Param        page query int false "page" default(1)
// @Param        search query string false "search"
// @Param        status query string false "status" Enums(all, waiting, running)
// @Success      200  {object}  types.Response{data=[]types.MirrorSyncSummary,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirrors [get]
func (h *MirrorHandler) Index(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req := types.MirrorSyncListReq{Page: page, Per: per, Search: ctx.Query("search")}
	switch status := strings.TrimSpace(ctx.Query("status")); status {
	case "", "all":
	case string(types.MirrorSyncOverallWaiting):
		req.Status = types.MirrorSyncOverallWaiting
	case string(types.MirrorSyncOverallRunning):
		req.Status = types.MirrorSyncOverallRunning
	default:
		httpbase.BadRequest(ctx, "status must be one of all, waiting, or running")
		return
	}
	result, err := h.mirror.ListMirrorSyncs(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to list mirror syncs", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  result.Items,
		"total": result.Total,
	}

	httpbase.OK(ctx, respData)
}

// GetMirrorStatusCounts godoc
// @Security     ApiKey
// @Summary      Get mirror status counts
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{data=[]types.MirrorStatusCount} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/statistics [get]
func (h *MirrorHandler) Statistics(ctx *gin.Context) {
	statusCounts, err := h.mirror.Statistics(ctx.Request.Context())
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get mirror statistics", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, statusCounts)
}

// DeleteMirrorTasks godoc
// @Security     ApiKey
// @Summary      Batch create mirrors
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        id path int false "id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/:id [delete]
func (h *MirrorHandler) Delete(ctx *gin.Context) {
	id := ctx.Param("id")
	idInt, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = h.mirror.Delete(ctx.Request.Context(), idInt)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to delete mirror tasks", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

// ResolveNamespace godoc
// @Security     ApiKey
// @Summary      Resolve target namespace and name from source
// @Tags         Mirror
// @Accept       json
// @Produce      json
// @Param        source_namespace query string true "source namespace"
// @Param        source_name query string true "source name"
// @Param        repo_type query string true "repo type (model, dataset, code)"
// @Param        target_namespace query string false "target namespace override"
// @Success      200  {object}  types.Response{data=types.ResolveNamespaceResp} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /mirror/namespace/resolve [get]
func (h *MirrorHandler) ResolveNamespace(ctx *gin.Context) {
	var req types.ResolveNamespaceReq
	err := ctx.ShouldBindQuery(&req)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	resp, err := h.mirror.ResolveNamespace(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to resolve namespace", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

// MirrorMetricsHandler serves the independent, read-only mirror telemetry API.
type MirrorMetricsHandler struct {
	read func(context.Context, types.MirrorMetricsQuery) (*types.MirrorMetricsResponse, error)
}

// NewMirrorMetricsHandler builds telemetry dependencies without requiring Redis
// availability at process startup. Outages are reported only by this endpoint.
func NewMirrorMetricsHandler(cfg *config.Config) *MirrorMetricsHandler {
	comp := component.NewMirrorMetricsComponent(cfg)
	return &MirrorMetricsHandler{read: comp.Get}
}

// Get godoc
// @Security ApiKey
// @Summary Read ephemeral mirror runtime metrics
// @Tags Mirror
// @Produce json
// @Param service query string false "all, repo, or lfs"
// @Param instance_id query string false "Instance UUID"
// @Param task_id query string false "Task ID"
// @Param page query int false "Task page" default(1)
// @Param per query int false "Tasks per page, at most 100" default(20)
// @Param instance_page query int false "Instance page" default(1)
// @Param instance_per query int false "Instances per page, at most 100" default(20)
// @Success 200 {object} types.Response{data=types.MirrorMetricsResponse}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 503 {object} types.APIInternalServerError
// @Router /admin/asset/mirror/metrics [get]
func (h *MirrorMetricsHandler) Get(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	// Reject malformed encoding before extracting supported metric filters.
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		httpbase.BadRequest(c, "invalid metrics query parameter")
		return
	}
	query, err := parseMirrorMetricsQuery(values)
	if err != nil {
		httpbase.BadRequest(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	result, err := h.read(ctx, query)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, httpbase.R{Msg: "mirror metrics cache unavailable"})
		return
	}
	httpbase.OK(c, result)
}

// parseMirrorMetricsQuery validates supported filters and ignores unrelated query parameters.
// Instance UUIDs are canonicalized; numeric IDs remain strings throughout the response contract.
func parseMirrorMetricsQuery(values url.Values) (types.MirrorMetricsQuery, error) {
	q := types.MirrorMetricsQuery{Service: "all", Page: 1, Per: 20, InstancePage: 1, InstancePer: 20}
	invalid := errors.New("invalid metrics query parameter")
	for _, name := range []string{"service", "instance_id", "task_id", "page", "per", "instance_page", "instance_per"} {
		if vals, present := values[name]; present && (len(vals) != 1 || vals[0] == "") {
			return q, invalid
		}
	}
	if v := values.Get("service"); v != "" {
		if v != "all" && v != "repo" && v != "lfs" {
			return q, invalid
		}
		q.Service = v
	}
	if v := values.Get("instance_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return q, invalid
		}
		q.InstanceID = id.String()
	}
	if v := values.Get("task_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 || strconv.FormatInt(id, 10) != v {
			return q, invalid
		}
		q.TaskID = v
	}
	for name, target := range map[string]*int{"page": &q.Page, "per": &q.Per, "instance_page": &q.InstancePage, "instance_per": &q.InstancePer} {
		if v := values.Get(name); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || (name == "per" || name == "instance_per") && n > 100 {
				return q, invalid
			}
			*target = n
		}
	}
	return q, nil
}
