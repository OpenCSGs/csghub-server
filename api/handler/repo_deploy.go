package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

// RuntimeFramework godoc
// @Security     ApiKey
// @Summary      [Deprecated: use GET:/{repo_type}/{namespace}/{name}/runtime_framework_v2 instead]
// @Description  List repo runtime framework
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param 		 deploy_type query int false "deploy_type" Enums(0, 1, 2, 4) default(1)
// @Success      200  {object}  types.Response{data=[]types.RuntimeFramework} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/runtime_framework [get]
func (h *RepoHandler) RuntimeFrameworkList(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	slog.Debug("list runtime framework", slog.Any("namespace", namespace), slog.Any("name", name))
	repoType := common.RepoTypeFromContext(ctx)
	if repoType == types.UnknownRepo {
		slog.ErrorContext(ctx.Request.Context(), "Bad request of repo type")
		httpbase.BadRequest(ctx, "Bad request of repo type")
		return
	}
	response, err := h.c.ListRuntimeFramework(ctx.Request.Context(), repoType, namespace, name, deployType)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "fail to list runtime framework", slog.String("error", err.Error()))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, response)
}

// RuntimeFramework godoc
// @Security     ApiKey
// @Summary      List repo runtime framework
// @Description  List repo runtime framework
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param 		 deploy_type query int false "deploy_type" Enums(0, 1, 2, 4) default(1)
// @Success      200  {object}  types.Response{data=[]types.RuntimeFrameworkV2} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/runtime_framework_v2 [get]
func (h *RepoHandler) RuntimeFrameworkListV2(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	slog.Debug("list runtime framework", slog.Any("namespace", namespace), slog.Any("name", name))
	repoType := common.RepoTypeFromContext(ctx)
	if repoType == types.UnknownRepo {
		slog.ErrorContext(ctx.Request.Context(), "Bad request of repo type")
		httpbase.BadRequest(ctx, "Bad request of repo type")
		return
	}
	response, err := h.c.ListRuntimeFrameworkV2(ctx.Request.Context(), repoType, namespace, name, deployType)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "fail to list runtime framework", slog.String("error", err.Error()))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, response)
}

// CreateRuntimeFramework godoc
// @Security     ApiKey
// @Summary      Create runtime framework
// @Description  create runtime framework
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param        body body types.RuntimeFrameworkReq true "body"
// @Success      200  {object}  types.RuntimeFramework "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework [post]
func (h *RepoHandler) RuntimeFrameworkCreate(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req types.RuntimeFrameworkReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.CurrentUser = currentUser
	frame, err := h.c.CreateRuntimeFramework(ctx.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to create runtime framework", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, frame)
}

// UpdateRuntimeFramework godoc
// @Security     ApiKey
// @Summary      Update runtime framework
// @Description  Update runtime framework
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path int true "id"
// @Param        current_user query string false "current user"
// @Param        body body types.RuntimeFrameworkReq true "body"
// @Success      200  {object}  types.RuntimeFramework "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id} [put]
func (h *RepoHandler) RuntimeFrameworkUpdate(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	var req types.RuntimeFrameworkReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	req.CurrentUser = currentUser
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request url format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	frame, err := h.c.UpdateRuntimeFramework(ctx.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to update runtime framework", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, frame)
}

// DeleteRuntimeFramework godoc
// @Security     ApiKey
// @Summary      Delete a exist RuntimeFramework
// @Description  delete a exist RuntimeFramework
// @Tags         RuntimeFramework
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path int true "id"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /runtime_framework/{id} [delete]
func (h *RepoHandler) RuntimeFrameworkDelete(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)

	err = h.c.DeleteRuntimeFramework(ctx.Request.Context(), currentUser, id)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete runtime framework", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// DeployList godoc
// @Security     ApiKey
// @Summary      List repo deploys
// @Description  List repo deploys
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{data=[]types.DeployRepo} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/run [get]
func (h *RepoHandler) DeployList(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	repoType := common.RepoTypeFromContext(ctx)
	response, err := h.c.ListDeploy(ctx.Request.Context(), repoType, namespace, name, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "fail to list deploy", slog.String("error", err.Error()), slog.Any("repotype", repoType), slog.Any("namespace", namespace), slog.Any("name", name))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, response)
}

// DeployDetail  godoc
// @Security     ApiKey
// @Summary      Get repo deploy detail
// @Description  Get repo deploy detail
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "id"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{data=types.DeployRepo} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Permission denied"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/run/{id} [get]
func (h *RepoHandler) DeployDetail(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	detailReq := types.DeployActReq{
		RepoType:    repoType,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    deployID,
	}
	switch repoType {
	case types.SpaceRepo:
		detailReq.DeployType = types.SpaceType
	case types.ModelRepo:
		detailReq.DeployType = types.InferenceType
	}

	response, err := h.c.DeployDetail(ctx.Request.Context(), detailReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Info("not allowed to get deploy detail", slog.Any("error", err), slog.Any("req", detailReq))
			httpbase.ForbiddenError(ctx, err)
		} else if errors.Is(err, errorx.ErrDatabaseNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			slog.ErrorContext(ctx.Request.Context(), "failed to get deploy detail", slog.Any("error", err), slog.Any("req", detailReq))
			httpbase.ServerError(ctx, err)
		}
		return
	}

	httpbase.OK(ctx, response)
}

// GetInferenceLogs   godoc
// @Security     ApiKey
// @Summary      get deploy instance logs
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "id"
// @Param        instance path string true "instance"
// @Param        current_user query string true "current_user"
// @Param        since query string false "since time. Optional values: 10mins, 30mins, 1hour, 6hours, 1day, 2days, 1week"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Permission denied"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/run/{id}/logs/{instance} [get]
func (h *RepoHandler) DeployInstanceLogs(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testLogs(ctx)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	instance := ctx.Query("instance")
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	logReq := types.DeployActReq{
		RepoType:     repoType,
		Namespace:    namespace,
		Name:         name,
		CurrentUser:  currentUser,
		DeployID:     deployID,
		DeployType:   types.InferenceType,
		InstanceName: instance,
		Since:        ctx.Query("since"),
	}

	// user http request context instead of gin context, so that server knows the life cycle of the request
	logReader, err := h.c.DeployInstanceLogs(ctx.Request.Context(), logReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Warn("not allowed to get instance logs", slog.Any("error", err), slog.Any("req", logReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "failed to get instance logs", slog.Any("error", err), slog.Any("req", logReq))
		httpbase.ServerError(ctx, err)
		return
	}

	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any deploy instance log"))
		return
	}

	// to quickly respond the http request
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("repo handler logs request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		case data, ok := <-logReader.RunLog():
			if ok {
				ctx.SSEvent("Container", string(data))
				ctx.Writer.Flush()
			}
		case <-heartbeatTicker.C:
			// Send a heartbeat message
			ctx.SSEvent("Heartbeat", "keep-alive")
			ctx.Writer.Flush()
		default:
			// Add a small sleep to prevent CPU spinning when no data is available
			time.Sleep(time.Second * 1)
		}
	}
}

// FinetuneInstanceLogs      godoc
// @Security     ApiKey
// @Summary      get finetune instance logs
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "id"
// @Param        instance path string true "instance"
// @Param        current_user query string true "current_user"
// @Param        since query string false "since time. Optional values: 10mins, 30mins, 1hour, 6hours, 1day, 2days, 1week"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Permission denied"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/finetune/{id}/logs/{instance} [get]
func (h *RepoHandler) FinetuneInstanceLogs(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testLogs(ctx)
		return
	}
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	instance := ctx.Query("instance")
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	logReq := types.DeployActReq{
		RepoType:     repoType,
		Namespace:    namespace,
		Name:         name,
		CurrentUser:  currentUser,
		DeployID:     deployID,
		DeployType:   types.FinetuneType,
		InstanceName: instance,
		Since:        ctx.Query("since"),
	}

	// user http request context instead of gin context, so that server knows the life cycle of the request
	logReader, err := h.c.DeployInstanceLogs(ctx.Request.Context(), logReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Warn("not allowed to get instance logs", slog.Any("error", err), slog.Any("req", logReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "failed to get instance logs", slog.Any("error", err), slog.Any("req", logReq))
		httpbase.ServerError(ctx, err)
		return
	}

	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any deploy instance log"))
		return
	}

	// to quickly respond the http request
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("repo handler logs request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		case data, ok := <-logReader.RunLog():
			if ok {
				ctx.SSEvent("Container", string(data))
				ctx.Writer.Flush()
			}
		case <-heartbeatTicker.C:
			// Send a heartbeat message
			ctx.SSEvent("Heartbeat", "keep-alive")
			ctx.Writer.Flush()
		default:
			// Add a small sleep to prevent CPU spinning when no data is available
			time.Sleep(time.Second * 1)
		}
	}
}

// GetDeployStatus   godoc
// @Security     JWT token
// @Summary      get deploy status
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "deploy id"
// @Param        current_user query string true "current_user"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Permission denied"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/run/{id}/status [get]
func (h *RepoHandler) DeployStatus(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testStatus(ctx)
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	statusReq := types.DeployActReq{
		RepoType:    repoType,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    deployID,
		DeployType:  types.InferenceType,
	}

	allow, err := h.c.AllowAccessDeploy(ctx.Request.Context(), statusReq)
	if err != nil {

		if errors.Is(err, errorx.ErrForbidden) {
			slog.Warn("not allowed to get deploy status", slog.Any("error", err), slog.Any("req", statusReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "failed to get deploy status", "error", err, "req", statusReq)
		httpbase.ServerError(ctx, err)
		return
	}

	if !allow {
		slog.ErrorContext(ctx.Request.Context(), "not allowed to query deploy status", "req", statusReq)
		httpbase.ForbiddenError(ctx, err)
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
			slog.Debug("deploy handler status request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		default:
			time.Sleep(h.deployStatusCheckInterval)
			// user http request context instead of gin context, so that server knows the life cycle of the request
			status, err := h.c.DeployStatus(ctx.Request.Context(), repoType, namespace, name, deployID)
			if err != nil {
				slog.ErrorContext(ctx.Request.Context(), "failed to get deploy status", slog.Any("error", err), slog.String("namespace", namespace),
					slog.String("name", name), slog.Any("deploy_id", deployID))
				ctx.SSEvent("error", err.Error())
			} else {
				ctx.SSEvent("status", status)

			}
			ctx.Writer.Flush()
		}
	}
}

func (h *RepoHandler) testStatus(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("deploy handler status request context done", slog.Any("error", ctx.Request.Context().Err()))
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

// DeployUpdate  godoc
// @Security     ApiKey
// @Summary      Update deploy parameters
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models" Enums(models)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "deploy id"
// @Param        current_user query string true "current_user"
// @Param        body body types.DeployUpdateReq true "deploy setting of inference"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/run/{id} [put]
func (h *RepoHandler) DeployUpdate(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	allow, err := h.c.AllowReadAccess(ctx.Request.Context(), types.ModelRepo, namespace, name, currentUser)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to check user permission", "error", err, slog.Any("currentUser", currentUser), slog.Any("namespace", name), slog.Any("name", name))
		httpbase.ServerError(ctx, errors.New("failed to check user permission"))
		return
	}

	if !allow {
		slog.Warn("user not allowed to update deploy", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("username", currentUser))
		httpbase.ForbiddenError(ctx, errors.New("user is not authorized to read this repository for update deploy"))
		return
	}

	var req *types.DeployUpdateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err, slog.Any("request.body", ctx.Request.Body))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if req.MinReplica != nil && req.MaxReplica != nil {
		err = Validate.Struct(req)
		if err != nil {
			slog.ErrorContext(ctx.Request.Context(), "Bad request setting for deploy", slog.Any("req", *req), slog.Any("err", err))
			httpbase.BadRequest(ctx, fmt.Sprintf("Bad request setting for deploy, %v", err))
			return
		}
	}

	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", slog.Any("error", err), slog.Any("id", ctx.Param("id")))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	updateReq := types.DeployActReq{
		RepoType:    repoType,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    deployID,
		DeployType:  types.InferenceType,
	}
	err = h.c.DeployUpdate(ctx.Request.Context(), updateReq, req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "user not allowed to update deploy", slog.String("namespace", namespace),
				slog.String("name", name), slog.Any("username", currentUser), slog.Int64("deploy_id", deployID))
			httpbase.ForbiddenError(ctx, err)
			return
		}
		slog.ErrorContext(ctx.Request.Context(), "failed to update deploy", slog.String("namespace", namespace), slog.String("name", name), slog.Any("username", currentUser), slog.Int64("deploy_id", deployID), slog.Any("error", err))
		httpbase.ServerError(ctx, fmt.Errorf("failed to update deploy, %w", err))
		return
	}

	httpbase.OK(ctx, nil)
}

// RuntimeFrameworkListWithType godoc
// @Security     ApiKey
// @Summary      List repo runtime framework
// @Description  List repo runtime framework
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models" Enums(models)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        current_user query string false "current user"
// @Param 		 deploy_type query int false "deploy_type" Enums(0, 1, 2) default(1)
// @Success      200  {object}  types.Response{data=[]types.RuntimeFramework} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/runtime_framework [get]
func (h *RepoHandler) RuntimeFrameworkListWithType(ctx *gin.Context) {
	deployTypeStr := ctx.Query("deploy_type")
	if deployTypeStr == "" {
		// backward compatibility for inferences
		deployTypeStr = strconv.Itoa(types.InferenceType)
	}
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	repoType := common.RepoTypeFromContext(ctx)
	if repoType == types.UnknownRepo {
		slog.ErrorContext(ctx.Request.Context(), "Bad request of repo type")
		httpbase.BadRequest(ctx, "Bad request of repo type")
		return
	}
	response, err := h.c.ListRuntimeFrameworkWithType(ctx.Request.Context(), deployType)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "fail to list runtime framework", slog.String("error", err.Error()))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, response)
}

// serverlessDetail  godoc
// @Security     ApiKey
// @Summary      Get repo serverless detail
// @Description  Get repo serverless detail
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "id"
// @Param        current_user query string false "current user"
// @Success      200  {object}  types.Response{data=types.DeployRepo} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless/{id} [get]
func (h *RepoHandler) ServerlessDetail(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	detailReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    deployID,
		DeployType:  types.ServerlessType,
	}

	response, err := h.c.DeployDetail(ctx.Request.Context(), detailReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "user not allowed to get serverless deploy detail", slog.String("namespace", namespace),
				slog.String("name", name), slog.Any("username", currentUser), slog.Int64("deploy_id", deployID))
			httpbase.ForbiddenError(ctx, err)
		} else if errors.Is(err, errorx.ErrDatabaseNoRows) {
			httpbase.NotFoundError(ctx, err)
		} else {
			slog.ErrorContext(ctx.Request.Context(), "fail to get serverless deploy detail", slog.String("error", err.Error()), slog.Any("namespace", namespace), slog.Any("name", name), slog.Any("deploy id", deployID))
			httpbase.ServerError(ctx, err)
		}
		return
	}

	httpbase.OK(ctx, response)
}

// GetServerlessLogs   godoc
// @Security     ApiKey
// @Summary      get serverless logs
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models" Enums(models)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "id"
// @Param        instance path string true "instance"
// @Param        current_user query string true "current_user"
// @Param        since query string false "since time. Optional values: 10mins, 30mins, 1hour, 6hours, 1day, 2days, 1week"
// @Failure      400  {object}  types.APIBadRequest "Bad request. May occur when the since time format is unsupported"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless/{id}/logs/{instance} [get]
func (h *RepoHandler) ServerlessLogs(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testLogs(ctx)
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	instance := ctx.Query("instance")
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	logReq := types.DeployActReq{
		RepoType:     repoType,
		Namespace:    namespace,
		Name:         name,
		CurrentUser:  currentUser,
		DeployID:     deployID,
		DeployType:   types.ServerlessType,
		InstanceName: instance,
		Since:        ctx.Query("since"),
	}

	// user http request context instead of gin context, so that server knows the life cycle of the request
	logReader, err := h.c.DeployInstanceLogs(ctx.Request.Context(), logReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "user not allowed to get serverless deploy logs", slog.Any("logReq", logReq), slog.Any("error", err))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "Failed to get serverless deploy logs", slog.Any("logReq", logReq), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any deploy instance log"))
		return
	}

	// to quickly respond the http request
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("repo handler logs request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		case data, ok := <-logReader.RunLog():
			if ok {
				ctx.SSEvent("Container", string(data))
				ctx.Writer.Flush()
			}
		case <-heartbeatTicker.C:
			// Send a heartbeat message
			ctx.SSEvent("Heartbeat", "keep-alive")
			ctx.Writer.Flush()
		default:
			// Add a small sleep to prevent CPU spinning when no data is available
			time.Sleep(time.Second * 1)
		}
	}
}

// GetServerlessStatus   godoc
// @Security     JWT token
// @Summary      get serverless status
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "deploy id"
// @Param        current_user query string true "current_user"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless/{id}/status [get]
func (h *RepoHandler) ServerlessStatus(ctx *gin.Context) {
	if ctx.Query("test") == "true" {
		h.testStatus(ctx)
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	statusReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    deployID,
		DeployType:  types.ServerlessType,
	}

	allow, err := h.c.AllowAccessDeploy(ctx.Request.Context(), statusReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "user not allowed to get serverless deploy status", slog.Any("error", err), slog.Any("req", statusReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "failed to check user permission", slog.Any("error", err), slog.Any("req", statusReq))
		httpbase.ServerError(ctx, fmt.Errorf("failed to check user permission, %w", err))
		return
	}

	if !allow {
		slog.Warn("user not allowed to query deploy status", slog.Any("req", statusReq))
		httpbase.ForbiddenError(ctx, errors.New("user not allowed to query serverless deploy status"))
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
			slog.Debug("deploy handler status request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		default:
			time.Sleep(h.deployStatusCheckInterval)
			// user http request context instead of gin context, so that server knows the life cycle of the request
			status, err := h.c.DeployStatus(ctx.Request.Context(), types.ModelRepo, namespace, name, deployID)
			if err != nil {
				slog.ErrorContext(ctx.Request.Context(), "failed to get deploy status", slog.Any("error", err), slog.String("namespace", namespace),
					slog.String("name", name), slog.Any("deploy_id", deployID))
				ctx.SSEvent("error", err.Error())
			} else {
				ctx.SSEvent("status", status)

			}
			ctx.Writer.Flush()
		}
	}
}

// ServerlessUpdate  godoc
// @Security     ApiKey
// @Summary      Update serverless parameters
// @Tags         Model
// @Accept       json
// @Produce      json
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "deploy id"
// @Param        current_user query string true "current_user"
// @Param        body body types.DeployUpdateReq true "deploy setting of Serverless"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /models/{namespace}/{name}/serverless/{id} [put]
func (h *RepoHandler) ServerlessUpdate(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace and name from context", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var req *types.DeployUpdateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err, slog.Any("request.body", ctx.Request.Body))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if req.MinReplica != nil && req.MaxReplica != nil {
		err = Validate.Struct(req)
		if err != nil {
			slog.ErrorContext(ctx.Request.Context(), "Bad request setting for serverless", slog.Any("req", *req), slog.Any("err", err))
			httpbase.BadRequest(ctx, fmt.Sprintf("Bad request setting for serverless, %v", err))
			return
		}
	}

	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", slog.Any("error", err), slog.Any("id", ctx.Param("id")))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	updateReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   namespace,
		Name:        name,
		CurrentUser: currentUser,
		DeployID:    deployID,
		DeployType:  types.ServerlessType,
	}
	err = h.c.DeployUpdate(ctx.Request.Context(), updateReq, req)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.Debug("user not allowed to update serverless", slog.Any("error", err), slog.Any("req", updateReq))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "failed to update serverless", slog.Any("error", err), slog.Any("req", updateReq))
		httpbase.ServerError(ctx, fmt.Errorf("failed to update serverless, %w", err))
		return
	}

	httpbase.OK(ctx, nil)
}

func (h *RepoHandler) testLogs(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		default:
			ctx.SSEvent("Container", "test run log message")
			ctx.Writer.Flush()
		}
		time.Sleep(time.Second * 5)
	}
}

// GetInferenceLogsByVersion   godoc
// @Security     ApiKey
// @Summary      get serverless logs by version (commitid)
// @Tags         Repository
// @Accept       json
// @Produce      json
// @Param        repo_type path string true "models,spaces" Enums(models,spaces)
// @Param        namespace path string true "namespace"
// @Param        name path string true "name"
// @Param        id path string true "id"
// @Param        commit_id path string true "commit_id"
// @Param        current_user query string true "current_user"
// @Param        since query string false "since time. Optional values: 10mins, 30mins, 1hour, 6hours, 1day, 2days, 1week"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      401  {object}  types.APIUnauthorized "Permission denied"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /{repo_type}/{namespace}/{name}/serverless/{id}/versions/{commit_id} [get]
func (h *RepoHandler) ServerlessVersionLogs(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get namespace and name from context", "error", err)
		httpbase.NotFoundError(ctx, err)
		return
	}

	currentUser := httpbase.GetCurrentUser(ctx)
	repoType := common.RepoTypeFromContext(ctx)
	deployID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		httpbase.BadRequest(ctx, "Invalid deploy ID format")
		return
	}
	commitID := ctx.Param("commit_id")
	instance := ctx.Query("instance_name")
	logReq := types.DeployActReq{
		RepoType:     repoType,
		Namespace:    namespace,
		Name:         name,
		CurrentUser:  currentUser,
		DeployID:     deployID,
		DeployType:   types.ServerlessType,
		InstanceName: instance,
		Since:        ctx.Query("since"),
		CommitID:     commitID,
	}

	logReader, err := h.c.DeployInstanceLogs(ctx.Request.Context(), logReq)
	if err != nil {
		if errors.Is(err, errorx.ErrForbidden) {
			slog.ErrorContext(ctx.Request.Context(), "user not allowed to get serverless deploy logs", slog.Any("logReq", logReq), slog.Any("error", err))
			httpbase.ForbiddenError(ctx, err)
			return
		}

		slog.ErrorContext(ctx.Request.Context(), "Failed to get serverless deploy logs", slog.Any("logReq", logReq), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any deploy instance log"))
		return
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")
	ctx.Writer.WriteHeader(http.StatusOK)
	ctx.Writer.Flush()

	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			slog.Debug("repo handler logs request context done", slog.Any("error", ctx.Request.Context().Err()))
			return
		case data, ok := <-logReader.RunLog():
			if ok {
				ctx.SSEvent("Container", string(data))
				ctx.Writer.Flush()
			}
		case <-heartbeatTicker.C:
			ctx.SSEvent("Heartbeat", "keep-alive")
			ctx.Writer.Flush()
		default:
			time.Sleep(time.Second * 1)
		}
	}
}
