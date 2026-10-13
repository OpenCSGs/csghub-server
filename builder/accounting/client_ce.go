//go:build !ee && !saas

package accounting

import (
	"opencsg.com/csghub-server/common/types"
)

func (ac *accountingClientImpl) QueryAllUsersBalance(per, page int) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) QueryBalanceByUserID(userUUID string) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) ListStatementByUserIDAndTime(req types.ActStatementsReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) ListBillsByUserIDAndDate(req types.ActStatementsReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) ListBillsDetailByUserID(req types.AcctBillsDetailReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) RechargeAccountingUser(userID string, req types.RechargeReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) PresentAccountingUser(userID string, req types.ActivityReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) CreateOrUpdateQuota(currentUser string, req types.AcctQuotaReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) GetQuotaByID(currentUser string) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) CreateQuotaStatement(currentUser string, req types.AcctQuotaStatementReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) GetQuotaStatement(currentUser string, req types.AcctQuotaStatementReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) QueryPricesBySKUType(currentUser string, req types.AcctPriceListReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) QueryPricesBySkuTypeAndKinds(currentUser string, req types.AcctPriceListByKindsReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) QueryDistinctPrices(currentUser string, req types.AcctPriceDistinctListReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) GetPriceByID(currentUser string, id int64) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) CreatePrice(currentUser string, req types.AcctPriceCreateReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) BatchCreatePrice(currentUser string, req types.AcctPriceBatchCreateReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) UpdatePrice(currentUser string, req types.AcctPriceUpdateReq, id int64) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) DeletePrice(currentUser string, id int64) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) CreateOrder(currentUser string, req types.AcctOrderCreateReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) ListRecharge(req types.AcctRechargeListReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) ListRecharges(req types.RechargesIndexReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) StatementsIndex(req types.ActStatementsReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) TokenReportIndex(req types.TokenReportReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) ListPresents(req types.PresentsIndexReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) GetOrderDetailByID(currentUser string, id int64) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) GetVoucherDashboard(req types.VoucherDashboardReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) OffLinePrice(req types.AcctPriceOffLineReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) QueryStatisticsByUserID(req types.AcctBillsReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) QueryStatisticsDetailByUserID(req types.AcctBillsDetailReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) GetStatisticsSummary(req types.AcctBillsReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) CreateTokenPlan(req types.AcctTokenPlanCreateReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) GetTokenPlan(req types.AcctTokenPlanGetReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) UpdateTokenPlan(req types.AcctTokenPlanUpdateReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) DeleteTokenPlan(req types.AcctTokenPlanDelReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) ListTokenPlans(req types.AcctTokenPlanListReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) SummaryTokenPlans() (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) ListPlansForNS(req types.AcctTokenPlanNSListReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) ListHistoryForNS(req types.AcctTokenPlanHistoryReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) PurchasePlanForNS(req types.AcctTokenPlanPurchaseReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) GetCurrentSubForNS(req types.AcctTokenPlanNSGetReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) CancelPlanForNS(req types.AcctTokenPlanCancelReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) UpgradePlanForNS(req types.AcctTokenPlanUpgradeReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) UpdatePlanForNS(req types.AcctTokenPlanUpdateNSReq) (any, error) {
	return nil, nil
}
