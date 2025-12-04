package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

func NewSpaceHandler(config *config.Config) (*SpaceHandler, error) {
	c, err := component.NewSpaceComponent(config)
	if err != nil {
		return nil, err
	}
	ssc, err := component.NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating sensitive component:%w", err)
	}
	repo, err := component.NewRepoComponent(config)
	if err != nil {
		return nil, fmt.Errorf("error creating repo component:%w", err)
	}
	return &SpaceHandler{
		space:                    c,
		sensitive:                ssc,
		repo:                     repo,
		spaceStatusCheckInterval: 5 * time.Second,
	}, nil
}

type SpaceHandler struct {
	space                    component.SpaceComponent
	sensitive                component.SensitiveComponent
	repo                     component.RepoComponent
	spaceStatusCheckInterval time.Duration
}

// GetAllSpaces   godoc
// @Security     ApiKey
// @Summary      Get spaces visible to current user
// @Description  get spaces visible to current user
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        current_user query string false "current user"
// @Param        search query string false "search text"
// @Param        task_tag query string false "filter by task tag"
// @Param        framework_tag query string false "filter by framework tag"
// @Param        license_tag query string false "filter by license tag"
// @Param        language_tag query string false "filter by language tag"
// @Param        sort query string false "sort by"
// @Param        source query string false "source" Enums(opencsg, huggingface, local)
// @Param        sdk query string false "filter by space sdk"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.ResponseWithTotal{data=[]types.Space,total=int} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces [get]
func (h *SpaceHandler) Index(ctx *gin.Context) {
	repoFilter := new(types.RepoFilter)
	repoFilter.Tags = parseTagReqs(ctx)
	repoFilter.Username = httpbase.GetCurrentUser(ctx)
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	repoFilter = getFilterFromContext(ctx, repoFilter)
	if !slices.Contains(types.Sorts, repoFilter.Sort) {
		msg := fmt.Sprintf("sort parameter must be one of %v", types.Sorts)
		err := errorx.ReqParamInvalid(errors.New(msg),
			errorx.Ctx().
				Set("param", "sort").
				Set("provided", repoFilter.Sort).
				Set("allowed", types.Sorts))
		slog.Error("Bad request format,", slog.String("error", msg))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	if repoFilter.Source != "" && !slices.Contains(types.Sources, repoFilter.Source) {
		msg := fmt.Sprintf("source parameter must be one of %v", types.Sources)
		err := errorx.ReqParamInvalid(errors.New(msg),
			errorx.Ctx().
				Set("param", "source").
				Set("provided", repoFilter.Source).
				Set("allowed", types.Sources))
		slog.Error("Bad request format,", slog.String("error", msg))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	repoFilter.SpaceSDK = ctx.Query("sdk")
	qNeedOpWeight := ctx.Query("need_op_weight")
	needOpWeight, err := strconv.ParseBool(qNeedOpWeight)
	if err != nil {
		needOpWeight = false
	}
	spaces, total, err := h.space.Index(ctx.Request.Context(), repoFilter, per, page, needOpWeight)
	if err != nil {
		slog.Error("Failed to get spaces", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Get public spaces succeed", slog.Int("count", total))
	httpbase.OKWithTotal(ctx, spaces, total)
}

// ShowSpaceDetail   godoc
// @Security     ApiKey
// @Summary      show space detail
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Param        need_op_weight query bool false "need op weight" default(false)
// @Success      200  {object}  types.Response{data=types.Space} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name} [get]
func (h *SpaceHandler) Show(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)

	qNeedOpWeight := ctx.Query("need_op_weight")
	needOpWeight, err := strconv.ParseBool(qNeedOpWeight)
	if err != nil {
		needOpWeight = false
	}

	detail, err := h.space.Show(ctx.Request.Context(), namespace, name, currentUser, needOpWeight)
	if err != nil {
		if errors.Is(err, errorx.ErrUnauthorized) {
			httpbase.UnauthorizedError(ctx, err)
			return
		}
		slog.Error("Failed to get space detail", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, detail)
}

// CreateSpace   godoc
// @Security     ApiKey
// @Summary      Create a new space
// @Description  create a new space
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        current_user query string true "current_user"
// @Param        body body types.CreateSpaceReq true "body"
// @Success      200  {object}  types.Response{data=types.Space} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces [post]
func (h *SpaceHandler) Create(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req types.CreateSpaceReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Username = currentUser
	if req.Namespace == "" {
		req.Namespace = currentUser
	}

	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), &req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}

	space, err := h.space.Create(ctx.Request.Context(), req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
		} else if errors.Is(err, errorx.ErrDatabaseDuplicateKey) {
			httpbase.BadRequestWithExt(ctx, err)
		} else {
			slog.Error("Failed to create space", slog.Any("error", err))
			httpbase.ServerError(ctx, err)
		}
		return
	}
	httpbase.OK(ctx, space)
}

// UpdateSpace   godoc
// @Security     ApiKey
// @Summary      Update a exists space
// @Description  update a exists space
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Param        body body types.UpdateSpaceReq true "body"
// @Success      200  {object}  types.Response{data=types.Space} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name} [put]
func (h *SpaceHandler) Update(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req *types.UpdateSpaceReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	_, err := h.sensitive.CheckRequestV2(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to check sensitive request", slog.Any("error", err))
		httpbase.BadRequest(ctx, fmt.Errorf("sensitive check failed: %w", err).Error())
		return
	}
	req.Username = currentUser

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.Namespace = namespace
	req.Name = name

	space, err := h.space.Update(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("Failed to update space", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	slog.Info("Update space succeed", slog.String("space", space.Name))
	httpbase.OK(ctx, space)
}

// DeleteSpace   godoc
// @Security     ApiKey
// @Summary      Delete a exists space
// @Description  delete a exists space
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name} [delete]
func (h *SpaceHandler) Delete(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = h.space.Delete(ctx.Request.Context(), namespace, name, currentUser)
	if err != nil {
		slog.Error("Failed to delete space", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	slog.Info("Delete space succeed", slog.String("space", name))
	httpbase.OK(ctx, nil)
}

// RunSpace   godoc
// @Security     JWT token
// @Summary      run space app
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  string "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/run [post]
func (h *SpaceHandler) Run(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	allow, err := h.repo.AllowAdminAccess(ctx.Request.Context(), types.SpaceRepo, namespace, name, currentUser)
	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}
	if !allow {
		slog.Warn("user not allowed to run space", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser))
		httpbase.ForbiddenError(ctx, errors.New("user not allowed to run space"))
		return
	}
	deployID, err := h.space.Deploy(ctx.Request.Context(), namespace, name, currentUser)
	if err != nil {
		slog.Error("failed to deploy space", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		if errors.Is(err, errorx.ErrNoEntryFile) {
			httpbase.BadRequestWithExt(ctx, err)
		} else {
			httpbase.ServerError(ctx, err)
		}
		return
	}

	slog.Info("space deployment created", slog.String("namespace", namespace),
		slog.String("name", name), slog.Int64("deploy_id", deployID))
	httpbase.OK(ctx, nil)
}

// WakeupSpace   godoc
// @Security     JWT token
// @Summary      wake up space app
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/wakeup [post]
func (h *SpaceHandler) Wakeup(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	err = h.space.Wakeup(ctx.Request.Context(), namespace, name)
	if err != nil {
		slog.Error("failed to wakeup space", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		if errors.Is(err, sql.ErrNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else if errors.Is(err, errorx.ErrNoEntryFile) {
			httpbase.BadRequestWithExt(ctx, err)
		} else {
			httpbase.ServerError(ctx, err)
		}
		return
	}

	httpbase.OK(ctx, nil)
}

// StopSpace   godoc
// @Security     JWT token
// @Summary      stop space app
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/stop [post]
func (h *SpaceHandler) Stop(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	allow, err := h.repo.AllowAdminAccess(ctx.Request.Context(), types.SpaceRepo, namespace, name, currentUser)
	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}
	if !allow {
		slog.Warn("user not allowed to stop space", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser))
		httpbase.ForbiddenError(ctx, errors.New("user not allowed to stop space"))
		return
	}

	err = h.space.Stop(ctx.Request.Context(), namespace, name, false)
	if err != nil {
		slog.Error("failed to stop space", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		if errors.Is(err, sql.ErrNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else if errors.Is(err, errorx.ErrNoEntryFile) {
			httpbase.BadRequestWithExt(ctx, err)
		} else {
			httpbase.ServerError(ctx, err)
		}
		return
	}

	slog.Info("stop space success", slog.String("namespace", namespace),
		slog.String("name", name))
	httpbase.OK(ctx, nil)
}

// GetSpaceStatus   godoc
// @Security     JWT token
// @Summary      get space status
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/status [get]
func (h *SpaceHandler) Status(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.status(ctx)
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	allow, err := h.repo.AllowReadAccess(ctx.Request.Context(), types.SpaceRepo, namespace, name, currentUser)
	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}

	if !allow {
		slog.Warn("user not allowed to query space status", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser))
		httpbase.ForbiddenError(ctx, errors.New("user not allowed to read space status"))
		return
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Info("space handler status request context done",
				slog.String("namespace", namespace), slog.String("name", name),
				slog.Any("error", ctx.Request.Context().Err()))
			return
		default:
			time.Sleep(h.spaceStatusCheckInterval)
			//user http request context instead of gin context, so that server knows the life cycle of the request
			_, status, err := h.space.Status(ctx.Request.Context(), namespace, name)
			if err != nil {
				if deadline, ok := ctx.Request.Context().Deadline(); ok {
					slog.Error("failed to get space status in stream", slog.Any("error", err),
						slog.String("namespace", namespace), slog.String("name", name),
						slog.Any("deadline", time.Until(deadline)))
				} else {
					slog.Error("failed to get space status in stream", slog.Any("error", err),
						slog.String("namespace", namespace), slog.String("name", name))
				}
				ctx.SSEvent("error", err.Error())
			} else {
				ctx.SSEvent("status", status)
			}
			ctx.Writer.Flush()
		}
	}
}

func (h *SpaceHandler) status(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Info("space handler status request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		default:
			time.Sleep(time.Second * 5)
			ctx.SSEvent("status", "Building")
			ctx.Writer.Flush()
			time.Sleep(time.Second * 5)
			ctx.SSEvent("status", "Running")
			ctx.Writer.Flush()
			time.Sleep(time.Second * 5)
			ctx.SSEvent("status", "Sleeping")
			ctx.Writer.Flush()
			time.Sleep(time.Second * 5)
			ctx.SSEvent("status", "Stopped")
			ctx.Writer.Flush()
		}
	}
}

// GetSpaceLogs   godoc
// @Security     JWT token
// @Summary      get space logs
// @Tags         Space
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string true "current_user"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /spaces/{namespace}/{name}/logs [get]
func (h *SpaceHandler) Logs(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testLogs(ctx)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	allow, err := h.repo.AllowReadAccess(ctx.Request.Context(), types.SpaceRepo, namespace, name, currentUser)
	if err != nil {
		slog.Error("failed to check user permission", "error", err)
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}

	if !allow {
		slog.Warn("user not allowed to read space logs", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser))
		httpbase.ForbiddenError(ctx, errors.New("user not allowed to read space logs"))
		return
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	//user http request context instead of gin context, so that server knows the life cycle of the request
	logReader, err := h.space.Logs(ctx.Request.Context(), namespace, name)
	if err != nil {
		if deadline, ok := ctx.Request.Context().Deadline(); ok {
			slog.Error("failed to get space logs",
				slog.Any("error", err),
				slog.String("namespace", namespace), slog.String("name", name),
				slog.Any("deadline", time.Until(deadline)), slog.Bool("ok", ok),
			)
		} else {
			slog.Error("failed to get space logs",
				slog.Any("error", err),
				slog.String("namespace", namespace), slog.String("name", name),
			)
		}

		httpbase.ServerError(ctx, err)
		return
	}

	if logReader.RunLog() == nil && logReader.BuildLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any space deploy log"))
		return
	}

	//to quickly respond the http request
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Info("space handler logs request context done",
				slog.String("namespace", namespace), slog.String("name", name),
				slog.Any("error", ctx.Request.Context().Err()))
			return
		case data, ok := <-logReader.BuildLog():
			if ok {
				ctx.SSEvent("Build", string(data))
				ctx.Writer.Flush()
			}
		case data, ok := <-logReader.RunLog():
			if ok {
				ctx.SSEvent("Container", string(data))
				ctx.Writer.Flush()
			}
		case <-heartbeatTicker.C:
			// Send a heartbeat event to keep the connection alive
			ctx.SSEvent("Heartbeat", "keep-alive")
			ctx.Writer.Flush()
		default:
			// Add a small sleep to prevent CPU spinning when no data is available
			time.Sleep(time.Second)
		}
	}
}

func (h *SpaceHandler) testLogs(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		default:
			ctx.SSEvent("Build", "test build log message")
			ctx.SSEvent("Container", "test run log message")
			ctx.Writer.Flush()
		}
		time.Sleep(time.Second * 5)
	}
}
