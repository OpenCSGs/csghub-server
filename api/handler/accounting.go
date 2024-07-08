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

// GetBalances   godoc
// @Security     ApiKey
// @Summary      Get all users balance
// @Description  Get all users balance
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        current_user query string true "current_user"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/credit/balance [get]
func (ah *AccountingHandler) QueryAllUsersBalance(ctx *gin.Context) {
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
	data, err := ah.ac.QueryAllUsersBalance(ctx, currentUser, per, page)
	if err != nil {
		errTip := "fail to get all accounts balance"
		slog.Error(errTip, slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New(errTip))
		return
	}
	httpbase.OK(ctx, data)
}

// GetUserBalance   godoc
// @Security     ApiKey
// @Summary      Get user balance by user uuid
// @Description  Get user balance by user uuid
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        id path string true "user uuid"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/credit/{id}/balance [get]
func (ah *AccountingHandler) QueryBalanceByUserID(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	userUUID := ctx.Param("id")
	if len(userUUID) < 1 {
		slog.Error("Bad request format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}
	data, err := ah.ac.QueryBalanceByUserID(ctx, currentUser, userUUID)
	if err != nil {
		errTip := "fail to get account balance"
		slog.Error(errTip, slog.Any("currentUser", currentUser), slog.Any("userUUID", userUUID), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New(errTip))
		return
	}
	httpbase.OK(ctx, data)
}

// ListStatement godoc
// @Security     ApiKey
// @Summary      List statements by user uuid and start time and end time
// @Description  List statements by user uuid and start time and end time
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
// @Router       /accounting/credit/{id}/statements [get]
func (ah *AccountingHandler) QueryStatementByUserID(ctx *gin.Context) {
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
	data, err := ah.ac.ListStatementByUserIDAndTime(ctx, req)
	if err != nil {
		errTip := "fail to query statement by user"
		slog.Error(errTip, slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New(errTip))
		return
	}
	httpbase.OK(ctx, data)
}

// ListBills     godoc
// @Security     ApiKey
// @Summary      List user bills by user uuid and start date and end date
// @Description  List user bills by user uuid and start date and end date
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        id path string true "user uuid"
// @Param 		 scene query int false "scene" Enums(10, 11, 12, 20) default(10)
// @Param        start_date query string true "start_date, format: '2024-06-12'"
// @Param        end_date query string true "end_date, format: '2024-07-12'"
// @Param        current_user query string true "current_user"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/credit/{id}/bills [get]
func (ah *AccountingHandler) QueryBillsByUserID(ctx *gin.Context) {
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
	startDate := ctx.Query("start_date") // format: '2024-06-12'
	endDate := ctx.Query("end_date")     // format: '2024-06-12'
	if len(startDate) < 1 || len(endDate) < 1 || len(userUUID) < 1 {
		slog.Error("Bad request format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}

	if !validateDateTimeFormat(startDate, "2006-01-02") || !validateDateTimeFormat(endDate, "2006-01-02") {
		slog.Error("Bad request date format")
		httpbase.BadRequest(ctx, "Bad request date format")
		return
	}
	req := types.ACCT_STATEMENTS_REQ{
		CurrentUser: currentUser,
		UserUUID:    userUUID,
		Scene:       scene,
		StartTime:   startDate,
		EndTime:     endDate,
		Per:         per,
		Page:        page,
	}
	data, err := ah.ac.ListBillsByUserIDAndDate(ctx, req)
	if err != nil {
		errTip := "fail to query bills by user"
		slog.Error(errTip, slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New(errTip))
		return
	}
	httpbase.OK(ctx, data)
}

// AccountRecharge godoc
// @Security     ApiKey
// @Summary      Recharge fee for account
// @Description  Recharge fee for account
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        id path string true "user uuid"
// @Param        current_user query string true "current_user"
// @Param        body body types.RECHARGE_REQ true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/credit/{id}/recharge [put]
func (ah *AccountingHandler) RechargeByUserID(ctx *gin.Context) {
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
	userUUID := ctx.Param("id")
	if len(userUUID) < 1 {
		slog.Error("Bad recharge user id")
		httpbase.BadRequest(ctx, "Bad recharge user id")
		return
	}

	data, err := ah.ac.RechargeAccountingUser(ctx, currentUser, userUUID, req)
	if err != nil {
		errTip := "fail to recharge user"
		slog.Error(errTip, slog.Any("currentUser", currentUser), slog.Any("userUUID", userUUID), slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New(errTip))
		return
	}

	httpbase.OK(ctx, data)
}

// CreateOrUpdateQuota  godoc
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
func (ah *AccountingHandler) CreateOrUpdateQuota(ctx *gin.Context) {
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

	data, err := ah.ac.CreateOrUpdateQuota(currentUser, req)
	if err != nil {
		errTip := "fail to add or update account quota"
		slog.Error(errTip, slog.Any("currentUser", currentUser), slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New(errTip))
		return
	}
	httpbase.OK(ctx, data)
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
func (ah *AccountingHandler) QueryQuota(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}

	data, err := ah.ac.GetQuotaByID(currentUser)
	if err != nil {
		errTip := "fail to get quota by user"
		slog.Error(errTip, slog.Any("currentUser", currentUser), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New(errTip))
		return
	}
	httpbase.OK(ctx, data)
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
// @Router       /accounting/multisync/downloads [post]
func (ah *AccountingHandler) CreateQuotaStatement(ctx *gin.Context) {
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

	data, err := ah.ac.CreateQuotaStatement(currentUser, req)
	if err != nil {
		errTip := "fail to create quota statement by user"
		slog.Error(errTip, slog.Any("currentUser", currentUser), slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New(errTip))
		return
	}
	httpbase.OK(ctx, data)
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
func (ah *AccountingHandler) QueryQuotaStatement(ctx *gin.Context) {
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

	data, err := ah.ac.GetQuotaStatement(currentUser, req)
	if err != nil {
		errTip := "fail to get account quota statement"
		slog.Error(errTip, slog.Any("currentUser", currentUser), slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, errors.New(errTip))
		return
	}

	httpbase.OK(ctx, data)
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

// GetPrices     godoc
// @Security     ApiKey
// @Summary      List sku prices
// @Description  List sku prices
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        sku_type query string true "sku_type" Enums(1, 2) default(1)
// @Param        resource_id query string true "resource_id"
// @Param        current_user query string true "current_user"
// @Param        per query int false "per" default(20)
// @Param        page query int false "per page" default(1)
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/price [get]
func (ah *AccountingHandler) QueryPricesBySKUTypeAndResourceID(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	skyStr := ctx.Query("sku_type")
	skuInt, err := strconv.Atoi(skyStr)
	if err != nil {
		slog.Error("Bad request sku type format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	resID := ctx.Query("resource_id")
	req := types.ACCT_PRICE_REQ{
		SKUType:    types.SKUType(skuInt),
		ResourceID: resID,
		Per:        per,
		Page:       page,
	}
	data, err := ah.ac.QueryPricesBySKUTypeAndResourceID(currentUser, req)
	if err != nil {
		slog.Error("fail to query prices by sku type and resource id", slog.Any("error", err), slog.Any("req", req))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, data)
}

// GetPrice      godoc
// @Security     ApiKey
// @Summary      Get price by id
// @Description  Get price by id
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        id path int true "id"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/price/{id} [get]
func (ah *AccountingHandler) GetPriceByID(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	idStr := ctx.Param("id")
	if len(idStr) < 1 {
		slog.Error("Bad request id format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Error("Bad request id format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	data, err := ah.ac.GetPriceByID(currentUser, int64(id))
	if err != nil {
		slog.Error("fail to query price by id", slog.Any("id", id), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, data)
}

// CreatePrice   godoc
// @Security     ApiKey
// @Summary      Add sku price
// @Description  Add sku price
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        current_user query string true "current_user"
// @Param        body body types.ACCT_PRICE true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/price [post]
func (ah *AccountingHandler) PriceCreate(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	var req types.ACCT_PRICE
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("Bad price request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	data, err := ah.ac.CreatePrice(currentUser, req)
	if err != nil {
		slog.Error("fail to add price", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, data)
}

// UpdatePrice   godoc
// @Security     ApiKey
// @Summary      Update sku price
// @Description  Update sku price
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        id path int true "id"
// @Param        current_user query string true "current_user"
// @Param        body body types.ACCT_PRICE true "body"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/price/{id} [put]
func (ah *AccountingHandler) PriceUpdate(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	idStr := ctx.Param("id")
	if len(idStr) < 1 {
		slog.Error("Bad request id format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Error("Bad request id format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	var req types.ACCT_PRICE
	err = ctx.ShouldBindJSON(&req)
	if err != nil {
		slog.Error("Bad price request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	data, err := ah.ac.UpdatePrice(currentUser, req, int64(id))
	if err != nil {
		slog.Error("fail to update price", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}

	httpbase.OK(ctx, data)
}

// DeletePrice   godoc
// @Security     ApiKey
// @Summary      Delete price by id
// @Description  Delete price by id
// @Tags         Accounting
// @Accept       json
// @Produce      json
// @Param        id path int true "id"
// @Param        current_user query string true "current_user"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  types.APIBadRequest "Bad request"
// @Failure      500  {object}  types.APIInternalServerError "Internal server error"
// @Router       /accounting/price/{id} [delete]
func (ah *AccountingHandler) PriceDelete(ctx *gin.Context) {
	currentUser := httpbase.GetCurrentUser(ctx)
	if currentUser == "" {
		httpbase.UnauthorizedError(ctx, errors.New("user not found, please login first"))
		return
	}
	idStr := ctx.Param("id")
	if len(idStr) < 1 {
		slog.Error("Bad request id format")
		httpbase.BadRequest(ctx, "Bad request format")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		slog.Error("Bad request id format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	data, err := ah.ac.DeletePrice(currentUser, int64(id))
	if err != nil {
		slog.Error("fail to delete price by id", slog.Any("id", id), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, data)
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
