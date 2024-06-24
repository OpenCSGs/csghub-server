package handler

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/types"
)

func NewMultiSyncHandler() (*MultiSyncHandler, error) {
	return &MultiSyncHandler{
		asqc:  component.NewAccountingQuotaComponent(),
		asqsc: component.NewAccountingQuotaStatementComponent(),
	}, nil
}

type MultiSyncHandler struct {
	asqc  *component.AccountingSyncQuotaComponent
	asqsc *component.AccountingSyncQuotaStatementComponent
}

func (msh *MultiSyncHandler) CreateOrUpdateQuota(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req types.ACCT_QUOTA_REQ
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("Bad quota request format", slog.Any("error", err))
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	quota, err := msh.asqc.CreateOrUpdateQuota(ctx, currentUser, req)
	if err != nil {
		slog.Error("fail to add or update account quota", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, quota)
}

func (msh *MultiSyncHandler) QueryQuota(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}

	quota, err := msh.asqc.GetQuotaByID(ctx, currentUser)
	if err != nil {
		slog.Error("fail to get quota by user", slog.Any("currentUser", currentUser), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, quota)
}

func (msh *MultiSyncHandler) CreateQuotaStatement(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req types.ACCT_QUOTA_STATEMENT_REQ
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("Bad quota statement request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	quotaSM, err := msh.asqsc.CreateQuotaStatement(ctx, currentUser, req)
	if err != nil {
		slog.Error("fail to create quota statement by user", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, quotaSM)
}

func (msh *MultiSyncHandler) QueryQuotaStatement(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}

	repoPath := ctx.Query("repo_path")
	repoType := ctx.Query("repo_type")
	req := types.ACCT_QUOTA_STATEMENT_REQ{
		RepoPath: repoPath,
		RepoType: repoType,
	}

	account, err := msh.asqsc.GetQuotaStatement(ctx, currentUser, req)
	if err != nil {
		slog.Error("fail to get account quota statement", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, account)
}
