package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/accounting/utils"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func NewMeteringHandler() (*MeteringHandler, error) {
	return &MeteringHandler{
		amc: component.NewMeteringComponent(),
	}, nil
}

type MeteringHandler struct {
	amc *component.MeteringComponent
}

func (mh *MeteringHandler) QueryMeteringStatementByUserID(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request pagination format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	scene, err := utils.GetSceneFromContext(ctx)
	if err != nil {
		slog.Error("Bad request scene format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	userID := ctx.Param("id")
	instance_name := ctx.Query("instance_name")
	startTime := ctx.Query("start_time") // format: '2024-06-12 08:27:22'
	endTime := ctx.Query("end_time")     // format: '2024-06-12 17:17:22'
	if len(startTime) < 1 || len(endTime) < 1 || len(userID) < 1 {
		slog.Error("Bad request parameters format")
		httpbase.BadRequest(ctx, "Bad request parameters format")
		return
	}
	if !utils.ValidateDateTimeFormat(startTime, "2006-01-02 15:04:05") || !utils.ValidateDateTimeFormat(endTime, "2006-01-02 15:04:05") {
		slog.Error("Bad request datetime format")
		httpbase.BadRequest(ctx, "Bad request datetime format")
		return
	}

	req := types.ACCT_STATEMENTS_REQ{
		UserUUID:     userID,
		Scene:        scene,
		InstanceName: instance_name,
		StartTime:    startTime,
		EndTime:      endTime,
		Per:          per,
		Page:         page,
	}

	meters, total, err := mh.amc.ListMeteringByUserIDAndDate(ctx, req)
	if err != nil {
		slog.Error("fail to query meters by user", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  meters,
		"total": total,
	}
	httpbase.OK(ctx, respData)
}
