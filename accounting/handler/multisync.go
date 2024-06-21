package handler

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/accounting/component"
	"opencsg.com/csghub-server/accounting/types"
	"opencsg.com/csghub-server/api/httpbase"
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

// CreateOrUpdateQuota      godoc
// @Security     ApiKey
// @Summary      Add or update account quota
// @Description  Add or update account quota
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        current_user query string true "current_user"
// @Param        body body types.ACCT_QUOTA_REQ true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/multisync/quotas [post]
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

// GetQuota      godoc
// @Security     ApiKey
// @Summary      Get account quota by user id
// @Description  Get account quota by user id
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/multisync/quota [get]
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

// AddDownload   godoc
// @Security     ApiKey
// @Summary      Add download count
// @Description  Add download count
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        current_user query string true "current_user"
// @Param        body body types.ACCT_QUOTA_STATEMENT_REQ true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/multisync/download [post]
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

// GetQuotaStatement     godoc
// @Security     ApiKey
// @Summary      Get account quota statement
// @Description  Get account quota statement
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        repo_path query string true "repo path"
// @Param        repo_type query string true "repo type"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/multisync/download [get]
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
	if err != nil || account == nil {
		slog.Error("fail to get account quota statement", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, account)
}
