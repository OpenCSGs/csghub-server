package handler

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
)

func NewCreditHandler(config *config.Config) (*CreditHandler, error) {
	return &CreditHandler{
		auc: component.NewAccountingUser(),
		asc: component.NewAccountingStatement(),
	}, nil
}

type CreditHandler struct {
	auc *component.AccountingUserComponent
	asc *component.AccountingStatementComponent
}

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
	account, err := ch.auc.ListAccountingByUserID(ctx, userID)
	if err != nil {
		slog.Error("fail to query account by user id", slog.Any("userid", userID), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, account)
}

func (ch *CreditHandler) QueryStatementByUserID(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
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
	statements, err := ch.asc.ListStatementByUserIDAndTime(ctx, userID, startTime, endTime)
	if err != nil {
		slog.Error("fail to query statement by user", slog.Any("userid", userID), slog.Any("start_time", startTime), slog.Any("end_time", endTime), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, statements)
}
