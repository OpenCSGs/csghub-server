package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

type AccountingHandler struct {
	ac       *component.AccountingComponent
	apiToken string
}

func NewAccountingHandler(config *config.Config) (*AccountingHandler, error) {
	acctComp, err := component.NewAccountingComponent(config)
	if err != nil {
		return nil, err
	}
	return &AccountingHandler{
		ac:       acctComp,
		apiToken: config.APIToken,
	}, nil
}

func validateDateTimeFormat(timeStr, layout string) bool {
	_, err := time.Parse(layout, timeStr)
	return err == nil
}

func getSceneFromContext(ctx *gin.Context) (int, error) {
	str := ctx.Query("scene")
	if str == "" {
		return 0, fmt.Errorf("bad request scene format")
	}
	scene, err := strconv.Atoi(str)
	return scene, err
}

// ListMeterings godoc
// @Security     ApiKey
// @Summary      List meterings by user uuid and start time and end time
// @Description  List meterings by user uuid and start time and end time
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        id path string true "user uuid"
// @Param 		 scene query int false "scene" Enums(10, 11, 12, 20) default(10)
// @Param        instance_name query string true "instance name"
// @Param        start_time query string true "start_time, format: '2024-06-12 08:27:22'"
// @Param        end_time query string true "end_time, format: '2024-06-12 17:17:22'"
// @Param        current_user query string true "current_user"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/metering/{id}/statements [get]
func (ah *AccountingHandler) QueryMeteringStatementByUserID(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	scene, err := getSceneFromContext(ctx)
	if err != nil {
		slog.Error("Bad request scene format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	userUUID := ctx.Param("id")
	instance_name := ctx.Query("instance_name")
	startTime := ctx.Query("start_time") // format: '2024-06-12 08:27:22'
	endTime := ctx.Query("end_time")     // format: '2024-06-12 17:17:22'
	if len(startTime) < 1 || len(endTime) < 1 || len(userUUID) < 1 {
		slog.Error("Bad request format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}
	if !validateDateTimeFormat(startTime, "2006-01-02 15:04:05") || !validateDateTimeFormat(endTime, "2006-01-02 15:04:05") {
		slog.Error("Bad request datetime format")
		httpbase.BadRequest(ctx, "Bad request datetime format")
		return
	}
	req := types.ACCT_STATEMENTS_REQ{
		CurrentUser:  currentUser,
		UserUUID:     userUUID,
		Scene:        scene,
		InstanceName: instance_name,
		StartTime:    startTime,
		EndTime:      endTime,
		Per:          per,
		Page:         page,
	}
	data, err := ah.ac.ListMeteringsByUserIDAndTime(ctx, req)
	if err != nil {
		errTip := "fail to query meterings by user"
		slog.Error(errTip, slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New(errTip))
		return
	}
	httpbase.OK(ctx, data)
}
