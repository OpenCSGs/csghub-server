package handler

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/accounting/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
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

// GetBalances   godoc
// @Security     ApiKey
// @Summary      Get all users balance
// @Description  Get all users balance
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/credit/balance [get]
func (ch *CreditHandler) QueryAllUsersBalance(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	accounts, err := ch.auc.ListAccountingUser(ctx)
	if err != nil {
		slog.Error("fail to list all accounts", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, accounts)
}

// GetUserBalance   godoc
// @Security     ApiKey
// @Summary      Get user balance by casdoor user uuid
// @Description  Get user balance by casdoor user uuid
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        id path int true "casdoor user uuid"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/credit/{id}/balance [get]
func (ch *CreditHandler) QueryBalanceByUserID(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	userID := ctx.Param("id")
	if len(userID) < 1 {
		slog.Error("Bad request format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}
	account, err := ch.auc.GetAccountingByUserID(ctx, userID)
	if err != nil || account == nil {
		slog.Error("fail to query account by user id", slog.Any("userid", userID), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, account)
}

// ListStatement   godoc
// @Security     ApiKey
// @Summary      List statements by casdoor user uuid and start time and end time
// @Description  List statements by casdoor user uuid and start time and end time
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        id path int true "casdoor user uuid"
// @Param        start_time query string true "start_time, format: '2024-06-12 08:27:22'"
// @Param        end_time query string true "end_time, format: '2024-06-12 17:17:22'"
// @Param        current_user query string true "current_user"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/credit/{id}/statements [get]
func (ch *CreditHandler) QueryStatementByUserID(ctx *gin.Context) {
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
	userID := ctx.Param("id")
	startTime := ctx.Query("start_time") // format: '2024-06-12 08:27:22'
	endTime := ctx.Query("end_time")     // format: '2024-06-12 17:17:22'
	if len(startTime) < 1 || len(endTime) < 1 || len(userID) < 1 {
		slog.Error("Bad request format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}
	if !validateDateTimeFormat(startTime, "2006-01-02 15:04:05") || !validateDateTimeFormat(endTime, "2006-01-02 15:04:05") {
		slog.Error("Bad request datetime format")
		httpbase.BadRequest(ctx, "Bad request datetime format")
		return
	}
	statements, err := ch.asc.ListStatementByUserIDAndTime(ctx, userID, startTime, endTime, per, page)
	if err != nil {
		slog.Error("fail to query statement by user", slog.Any("userid", userID), slog.Any("start_time", startTime), slog.Any("end_time", endTime), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, statements)
}

// ListBills     godoc
// @Security     ApiKey
// @Summary      List user bills by casdoor user uuid and start date and end date
// @Description  List user bills by casdoor user uuid and start date and end date
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        id path int true "casdoor user uuid"
// @Param        start_date query string true "start_date, format: '2024-06-12'"
// @Param        end_date query string true "end_date, format: '2024-07-12'"
// @Param        current_user query string true "current_user"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/credit/{id}/bills [get]
func (ch *CreditHandler) QueryBillsByUserID(ctx *gin.Context) {
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
	userID := ctx.Param("id")
	startDate := ctx.Query("start_date") // format: '2024-06-12'
	endDate := ctx.Query("end_date")     // format: '2024-06-12'
	if len(startDate) < 1 || len(endDate) < 1 || len(userID) < 1 {
		slog.Error("Bad request format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}

	if !validateDateTimeFormat(startDate, "2006-01-02") || !validateDateTimeFormat(endDate, "2006-01-02") {
		slog.Error("Bad request date format")
		httpbase.BadRequest(ctx, "Bad request date format")
		return
	}

	bills, err := ch.abc.ListBillsByUserIDAndDate(ctx, userID, startDate, endDate, per, page)
	if err != nil {
		slog.Error("fail to query bills by user", slog.Any("userid", userID), slog.Any("start_date", startDate), slog.Any("end_date", endDate), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, bills)
}

func validateDateTimeFormat(timeStr, layout string) bool {
	_, err := time.Parse(layout, timeStr)
	return err == nil
}

// AccountRecharge     godoc
// @Security     ApiKey
// @Summary      Recharge fee for account
// @Description  Recharge fee for account
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        id path int true "casdoor user uuid"
// @Param        current_user query string true "current_user"
// @Param        body body types.RECHARGE_REQ true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/credit/{id}/recharge [put]
func (ch *CreditHandler) RechargeByUserID(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
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
	if req.OpUID < 0 {
		slog.Error("Bad operate user id")
		httpbase.BadRequest(ctx, "Bad operate user id")
		return
	}
	userID := ctx.Param("id")
	if len(userID) < 1 {
		slog.Error("Bad recharge user id")
		httpbase.BadRequest(ctx, "Bad recharge user id")
		return
	}
	err = ch.auc.CheckAccountingUser(ctx, userID)
	if err != nil {
		slog.Error("fail to check user balance", slog.Any("userID", userID), slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	err = ch.asc.RechargeAccountingUser(ctx, userID, req)
	if err != nil {
		slog.Error("fail to recharge account by user", slog.Any("userID", userID), slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	account, err := ch.auc.GetAccountingByUserID(ctx, userID)
	if err != nil || account == nil {
		slog.Error("fail to get account by user id", slog.Any("userID", userID), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, account)
}
