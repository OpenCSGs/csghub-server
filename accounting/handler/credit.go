package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/accounting/utils"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func NewCreditHandler(config *config.Config) (*CreditHandler, error) {
	return &CreditHandler{
		auc: component.NewAccountingUser(),
		asc: component.NewAccountingStatement(),
		abc: component.NewAccountingBill(),
	}, nil
}

type CreditHandler struct {
	auc *component.AccountingUserComponent
	asc *component.AccountingStatementComponent
	abc *component.AccountingBillComponent
}

func (ch *CreditHandler) QueryAllUsersBalance(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	accounts, total, err := ch.auc.ListAccountingUser(ctx, per, page)
	if err != nil {
		slog.Error("fail to list all accounts", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  accounts,
		"total": total,
	}
	httpbase.OK(ctx, respData)
}

func (ch *CreditHandler) QueryBalanceByUserID(ctx *gin.Context) {
	userUUID := ctx.Param("id")
	if len(userUUID) < 1 {
		slog.Error("Bad request format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}
	account, err := ch.auc.GetAccountingByUserID(ctx, userUUID)
	if err != nil || account == nil {
		slog.Error("fail to query account by user id", slog.Any("userid", userUUID), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, account)
}

func (ch *CreditHandler) QueryStatementByUserID(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
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
		slog.Error("Bad request format")
		httpbase.BadRequest(ctx, "Bad request format")
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

	respData, err := ch.asc.ListStatementByUserIDAndTime(ctx, req)
	if err != nil {
		slog.Error("fail to query statement by user", slog.Any("userid", userID), slog.Any("start_time", startTime), slog.Any("end_time", endTime), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, respData)
}

func (ch *CreditHandler) QueryBillsByUserID(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	scene, err := utils.GetSceneFromContext(ctx)
	if err != nil {
		slog.Error("Bad request scene format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	userUUID := ctx.Param("id")
	startDate := ctx.Query("start_date") // format: '2024-06-12'
	endDate := ctx.Query("end_date")     // format: '2024-06-12'
	if len(startDate) < 1 || len(endDate) < 1 || len(userUUID) < 1 {
		slog.Error("Bad request format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}

	if !utils.ValidateDateTimeFormat(startDate, "2006-01-02") || !utils.ValidateDateTimeFormat(endDate, "2006-01-02") {
		slog.Error("Bad request date format")
		httpbase.BadRequest(ctx, "Bad request date format")
		return
	}

	req := types.ACCT_BILLS_REQ{
		UserUUID:  userUUID,
		Scene:     scene,
		StartDate: startDate,
		EndDate:   endDate,
		Per:       per,
		Page:      page,
	}

	respData, err := ch.abc.ListBillsByUserIDAndDate(ctx, req)
	if err != nil {
		slog.Error("fail to query bills by user", slog.Any("userUUID", userUUID), slog.Any("start_date", startDate), slog.Any("end_date", endDate), slog.Any("scene", scene), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, respData)
}

func (ch *CreditHandler) RechargeByUserID(ctx *gin.Context) {
	var req types.RECHARGE_REQ
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("Bad recharge request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	if req.Value < 0 {
		slog.Error("Bad recharge value")
		httpbase.BadRequest(ctx, "Bad recharge value")
		return
	}
	userUUID := ctx.Param("id")
	if len(userUUID) < 1 {
		slog.Error("Bad recharge user id")
		httpbase.BadRequest(ctx, "Bad recharge user id")
		return
	}
	err = ch.auc.CheckAccountingUser(ctx, userUUID)
	if err != nil {
		slog.Error("fail to check user balance", slog.Any("user_uuid", userUUID), slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = ch.asc.RechargeAccountingUser(ctx, userUUID, req)
	if err != nil {
		slog.Error("fail to recharge account by user", slog.Any("user_uuid", userUUID), slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	account, err := ch.auc.GetAccountingByUserID(ctx, userUUID)
	if err != nil || account == nil {
		slog.Error("fail to get account by user id", slog.Any("user_uuid", userUUID), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, account)
}
