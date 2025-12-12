package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func NewNotebookHandler(config *config.Config) (*NotebookHandler, error) {
	nc, err := component.NewNotebookComponent(config)
	if err != nil {
		return nil, err
	}
	return &NotebookHandler{
		nc:     nc,
		config: config,
	}, nil
}

type NotebookHandler struct {
	nc     component.NotebookComponent
	config *config.Config
}

// CreateNotebook godoc
// @Summary      Create notebook
// @Description  create notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        body body types.CreateNotebookReq false "body"
// @Success      200  {object}  types.NotebookRes "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks [post]
func (h *NotebookHandler) Create(ctx *gin.Context) {
	var req types.CreateNotebookReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to bind json", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req.CurrentUser = currentUser
	notebook, err := h.nc.CreateNotebook(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to create notebook", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, notebook)
}

// GetNotebook godoc
// @Summary      Get notebook
// @Description  get notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Success      200  {object}  types.NotebookRes "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks/{id} [get]
func (h *NotebookHandler) Get(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	req := &types.GetNotebookReq{
		CurrentUser: currentUser,
		ID:          id,
	}

	notebook, err := h.nc.GetNotebook(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to get notebook", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, notebook)
}

// StartNotebook godoc
// @Summary      Start notebook
// @Description  start notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks/{id}/start [put]
func (h *NotebookHandler) Start(ctx *gin.Context) {

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.StartNotebookReq{
		CurrentUser: currentUser,
		ID:          id,
	}
	err = h.nc.StartNotebook(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to start notebook", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// StopNotebook godoc
// @Summary      Stop notebook
// @Description  stop notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks/{id}/stop [put]
func (h *NotebookHandler) Stop(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.StopNotebookReq{
		CurrentUser: currentUser,
		ID:          id,
	}
	err = h.nc.StopNotebook(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to stop notebook", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// WakeupNotebook godoc
// @Summary      Wakeup notebook
// @Description  Wakeup notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks/{id}/wakeup [put]
func (h *NotebookHandler) Wakeup(ctx *gin.Context) {
	var (
		id  int64
		err error
	)
	id, err = strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err = errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	err = h.nc.Wakeup(ctx.Request.Context(), id)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to wakeup notebook", slog.String("id", string(rune(id))), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New("failed to wakeup notebook"))
		return
	}
	httpbase.OK(ctx, nil)
}

// DeleteNotebook godoc
// @Summary      Delete notebook
// @Description  delete notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks/{id} [delete]
func (h *NotebookHandler) Delete(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.DeleteNotebookReq{
		CurrentUser: currentUser,
		ID:          id,
	}
	err = h.nc.DeleteNotebook(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to delete notebook", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// UpdateNotebook godoc
// @Summary      Update notebook
// @Description  update notebook
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Param        body body types.UpdateNotebookReq false "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /notebooks/{id} [put]
func (h *NotebookHandler) Update(ctx *gin.Context) {
	var req types.UpdateNotebookReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to bind json", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	req.ID = id
	currentUser := httpbase.GetCurrentUser(ctx)
	req.CurrentUser = currentUser
	err = h.nc.UpdateNotebook(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to update notebook", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// GetNotebookStatus   godoc
// @Security     JWT token
// @Summary      get notebook status
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router        /notebooks/{id}/status [get]
func (h *NotebookHandler) Status(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	req := &types.StatusNotebookReq{
		CurrentUser: currentUser,
		ID:          id,
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
			slog.Info("notebook handler status request context done",
				slog.String("notebook id", strconv.FormatInt(id, 10)))
			return
		default:
			time.Sleep(5 * time.Second)
			//user http request context instead of gin context, so that server knows the life cycle of the request
			status, err := h.nc.StatusNotebook(ctx.Request.Context(), req)
			if err != nil {
				if deadline, ok := ctx.Request.Context().Deadline(); ok {
					slog.ErrorContext(ctx.Request.Context(), "failed to get notebook status in stream", slog.Any("error", err),
						slog.String("notebook id", strconv.FormatInt(id, 10)),
						slog.Any("deadline", time.Until(deadline)))
				} else {
					slog.ErrorContext(ctx.Request.Context(), "failed to get notebook status in stream", slog.Any("error", err),
						slog.String("notebook id", strconv.FormatInt(id, 10)))
				}
				ctx.SSEvent("error", err.Error())
			} else {
				ctx.SSEvent("status", status)
			}
			ctx.Writer.Flush()
		}
	}
}

// GetNotebookLogs   godoc
// @Security     JWT token
// @Summary      get notebook logs
// @Tags         Notebook
// @Accept       json
// @Produce      json
// @Param        id path string true "notebook id"
// @Param        instance path string true "notebook instance name"
// @Param        since query string false "since time. Optional values: 10mins, 30mins, 1hour, 6hours, 1day, 2days, 1week"
// @Failure      400  {object}  types.APIBadRequest "Bad request. May occur when the since time format is unsupported"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router        /notebooks/{id}/logs/{instance} [get]
func (h *NotebookHandler) Logs(ctx *gin.Context) {

	currentUser := httpbase.GetCurrentUser(ctx)
	instance := ctx.Param("instance")
	if len(instance) < 1 {
		err := errorx.ReqParamInvalid(errors.New("instance name is required"), errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Bad request format", "error", err)
		err := errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "id"))
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	req := &types.StatusNotebookReq{
		CurrentUser:  currentUser,
		ID:           id,
		InstanceName: instance,
	}

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")

	//user http request context instead of gin context, so that server knows the life cycle of the request
	logReader, err := h.nc.LogsNotebook(ctx.Request.Context(), req)
	if err != nil {
		if deadline, ok := ctx.Request.Context().Deadline(); ok {
			slog.ErrorContext(ctx.Request.Context(), "failed to get space logs",
				slog.Any("error", err),
				slog.String("notebook id", strconv.FormatInt(id, 10)),
				slog.Any("deadline", time.Until(deadline)), slog.Bool("ok", ok),
			)
		} else {
			slog.ErrorContext(ctx.Request.Context(), "failed to get space logs",
				slog.Any("error", err),
				slog.String("notebook id", strconv.FormatInt(id, 10)),
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
				slog.String("notebook id", strconv.FormatInt(id, 10)),
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
