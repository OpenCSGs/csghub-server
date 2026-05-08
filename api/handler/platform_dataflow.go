package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

type PlatformDataflowHandler struct {
	component component.PlatformDataflowComponent
}

func NewPlatformDataflowHandler(config *config.Config) (*PlatformDataflowHandler, error) {
	c, err := component.NewPlatformDataflowComponent(config)
	if err != nil {
		return nil, err
	}
	return &PlatformDataflowHandler{
		component: c,
	}, nil
}

func (h *PlatformDataflowHandler) CreateJob(ctx *gin.Context) {
	currentUserName := httpbase.GetCurrentUser(ctx)
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	nsUUID := ctx.Param("uuid")

	if len(nsUUID) < 1 {
		err := fmt.Errorf("ns_uuid is required")
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	var req types.DataflowArgoJobReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to bind request body", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req.OpUserUUID = currentUserUUID
	req.Username = currentUserName
	req.NSUUID = nsUUID

	resp, err := h.component.CreateJob(ctx.Request.Context(), &req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to create dataflow workflow job",
			slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.Created(ctx, resp)
}

func (h *PlatformDataflowHandler) DeleteJob(ctx *gin.Context) {
	currentUserName := httpbase.GetCurrentUser(ctx)
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	taskID := ctx.Param("task_id")
	nsUUID := ctx.Param("uuid")
	if len(taskID) < 1 {
		err := fmt.Errorf("task_id is required")
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if len(nsUUID) < 1 {
		err := fmt.Errorf("ns_uuid is required")
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req := &types.DataflowDeleteReq{
		OpUserUUID: currentUserUUID,
		Username:   currentUserName,
		ArgoTaskID: taskID,
		NSUUID:     nsUUID,
	}

	err := h.component.DeleteJob(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to delete dataflow workflow job",
			slog.Any("error", err), slog.String("taskid", taskID), slog.String("nsuuid", nsUUID))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, nil)
}

func (h *PlatformDataflowHandler) GetJob(ctx *gin.Context) {
	currentUserName := httpbase.GetCurrentUser(ctx)
	currentUserUUID := httpbase.GetCurrentUserUUID(ctx)
	taskID := ctx.Param("task_id")
	nsUUID := ctx.Param("uuid")

	if len(taskID) < 1 {
		err := fmt.Errorf("task_id is required")
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	if len(nsUUID) < 1 {
		err := fmt.Errorf("ns_uuid is required")
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req := &types.DataflowArgoJobReq{
		OpUserUUID: currentUserUUID,
		Username:   currentUserName,
		NSUUID:     nsUUID,
		ArgoTaskID: taskID,
	}

	resp, err := h.component.GetJob(ctx.Request.Context(), req)
	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "failed to get dataflow workflow job",
			slog.Any("error", err), slog.String("taskid", taskID))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, resp)
}

func (h *PlatformDataflowHandler) GetLogs(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)

	since := ctx.Query("since")
	stream := ctx.Query("stream")
	taskID := ctx.Param("task_id")
	dagTaskID := ctx.Query("dag_task_id")

	if len(taskID) < 1 {
		err := fmt.Errorf("task_id is required")
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	req := types.DataflowLogReq{
		CurrentUser: currentUser,
		Since:       since,
		TaskId:      taskID,
		DagTaskId:   dagTaskID,
	}

	allow, err := h.component.CheckUserPermission(ctx.Request.Context(), req)
	if !allow {
		slog.Error("user not allowed to read dataflow job logs", slog.Any("error", err), slog.Any("req", req))
		httpbase.ForbiddenError(ctx, errors.New("user not allowed to read dataflow job logs"))
		return
	}

	if strings.Trim(stream, " ") == "true" {
		h.readLogInStream(ctx, req)
	} else {
		h.readLogNonStream(ctx, req)
	}
}

func (h *PlatformDataflowHandler) readLogNonStream(ctx *gin.Context, req types.DataflowLogReq) {
	logs, err := h.component.ReadJobLogsNonStream(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to get dataflow job non-stream logs", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, logs)
}

func (h *PlatformDataflowHandler) readLogInStream(ctx *gin.Context, req types.DataflowLogReq) {
	logReader, err := h.component.ReadJobLogsInStream(ctx.Request.Context(), req)
	if err != nil {
		slog.Error("failed to get dataflow job in-stream logs", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	if logReader.RunLog() == nil {
		httpbase.ServerError(ctx, errors.New("don't find any dataflow job log"))
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
