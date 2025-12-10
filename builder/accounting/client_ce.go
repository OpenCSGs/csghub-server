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

func (ac *accountingClientImpl) GetPriceByID(currentUser string, id int64) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) CreatePrice(currentUser string, req types.AcctPriceCreateReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) UpdatePrice(currentUser string, req types.AcctPriceCreateReq, id int64) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) DeletePrice(currentUser string, id int64) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) CreateOrder(currentUser string, req types.AcctOrderCreateReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) ListRechargeByUserIDAndTime(req types.AcctRechargeListReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) ListRecharges(req types.RechargesIndexReq) (any, error) {
	return nil, nil
}

func (ac *accountingClientImpl) StatementsIndex(req types.ActStatementsReq) (any, error) {
	return nil, nil
}
