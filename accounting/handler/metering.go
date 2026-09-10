package handler

import (
	"errors"
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
	amc component.MeteringComponent
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

	req := types.ActStatementsReq{
		UserUUID:     userID,
		Scene:        types.SceneType(scene),
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

func (mh *MeteringHandler) QueryMeteringStatByDate(ctx *gin.Context) {
	scene, err := utils.GetSceneFromContext(ctx)
	if err != nil {
		slog.Error("Bad request scene format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	startDate := ctx.Query("start_date") // format: '2024-06-12'
	endDate := ctx.Query("end_date")     // format: '2024-06-12'

	if !utils.ValidateDateTimeFormat(startDate, "2006-01-02") || !utils.ValidateDateTimeFormat(endDate, "2006-01-02") {
		slog.Error("Bad request date format")
		httpbase.BadRequest(ctx, "Bad request date format")
		return
	}

	req := types.ActStatementsReq{
		Scene:     types.SceneType(scene),
		StartTime: startDate + " 00:00:00",
		EndTime:   endDate + " 23:59:59",
	}

	res, err := mh.amc.GetMeteringStatByDate(ctx, req)
	if err != nil {
		slog.Error("fail to query meter stat", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, res)
}

func (mh *MeteringHandler) QueryStatisticsByUserID(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	scene, err := utils.GetSceneFromContext(ctx)
	if err != nil {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	userUUID := ctx.Param("id")
	instanceName := ctx.Query("instance_name")
	startDate := ctx.Query("start_date") // format: '2024-06-12'
	endDate := ctx.Query("end_date")     // format: '2024-06-12'
	if len(startDate) < 1 || len(endDate) < 1 || len(userUUID) < 1 {
		httpbase.BadRequestWithExt(ctx, errors.New("bad data format"))
		return
	}

	if !utils.ValidateDateTimeFormat(startDate, "2006-01-02") || !utils.ValidateDateTimeFormat(endDate, "2006-01-02") {
		httpbase.BadRequestWithExt(ctx, errors.New("bad data format"))
		return
	}

	req := types.AcctBillsReq{
		TargetUUID:   userUUID,
		Scene:        types.SceneType(scene),
		StartDate:    startDate,
		EndDate:      endDate,
		Per:          per,
		Page:         page,
		InstanceName: instanceName,
	}

	respData, err := mh.amc.ListStatisticsByUserIDAndDate(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to query statistics by user",
			slog.Any("userUUID", userUUID),
			slog.Any("start_date", startDate),
			slog.Any("end_date", endDate),
			slog.Any("scene", scene),
			slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, respData)
}

func (mh *MeteringHandler) QueryStatisticsDetailByUserID(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}
	scene, err := utils.GetSceneFromContext(ctx)
	if err != nil {
		httpbase.BadRequestWithExt(ctx, err)
		return
	}

	targetUUID := ctx.Param("id")
	instanceName := ctx.Query("instance_name")
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")
	if len(startDate) < 1 || len(endDate) < 1 || len(targetUUID) < 1 {
		httpbase.BadRequestWithExt(ctx, errors.New("bad data format"))
		return
	}

	if !utils.ValidateDateTimeFormat(startDate, "2006-01-02") || !utils.ValidateDateTimeFormat(endDate, "2006-01-02") {
		httpbase.BadRequestWithExt(ctx, errors.New("bad data format"))
		return
	}
	req := types.AcctBillsDetailReq{
		TargetUUID:   targetUUID,
		Scene:        scene,
		StartDate:    startDate,
		EndDate:      endDate,
		InstanceName: instanceName,
		Per:          per,
		Page:         page,
	}

	respData, err := mh.amc.ListStatisticsDetailByUserID(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to query statistics detail by user", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, respData)
}

func (mh *MeteringHandler) QueryStatisticsSummary(ctx *gin.Context) {
	scene, _ := utils.GetSceneFromContext(ctx)
	userUUID := ctx.Param("id")
	startDate := ctx.Query("start_date")
	endDate := ctx.Query("end_date")
	if len(startDate) < 1 || len(endDate) < 1 || len(userUUID) < 1 {
		httpbase.BadRequestWithExt(ctx, errors.New("bad data format"))
		return
	}

	if !utils.ValidateDateTimeFormat(startDate, "2006-01-02") || !utils.ValidateDateTimeFormat(endDate, "2006-01-02") {
		httpbase.BadRequestWithExt(ctx, errors.New("bad data format"))
		return
	}

	req := types.AcctBillsReq{
		TargetUUID: userUUID,
		Scene:      types.SceneType(scene),
		StartDate:  startDate,
		EndDate:    endDate,
	}

	respData, err := mh.amc.GetStatisticsSummary(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to query statistics summary",
			slog.Any("userUUID", userUUID),
			slog.Any("start_date", startDate),
			slog.Any("end_date", endDate),
			slog.Any("scene", scene),
			slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, respData)
}
