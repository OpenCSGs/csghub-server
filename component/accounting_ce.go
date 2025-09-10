//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (ac *accountingComponentImpl) QueryAllUsersBalance(ctx context.Context, per, page int) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) QueryBalanceByUserID(ctx context.Context, currentUser, userUUID string) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) QueryBalanceByUserIDInternal(ctx context.Context, currentUser string) (*database.AccountUser, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) ListStatementByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) ListBillsByUserIDAndDate(ctx context.Context, req types.ActStatementsReq) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) RechargeAccountingUser(ctx context.Context, userUUID string, req types.RechargeReq) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) CreateOrUpdateQuota(currentUser string, req types.AcctQuotaReq) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) GetQuotaByID(currentUser string) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) CreateQuotaStatement(currentUser string, req types.AcctQuotaStatementReq) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) GetQuotaStatement(currentUser string, req types.AcctQuotaStatementReq) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) QueryPricesBySKUType(currentUser string, req types.AcctPriceListReq) (*database.PriceResp, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) GetPriceByID(currentUser string, id int64) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) CreatePrice(currentUser string, req types.AcctPriceCreateReq) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) UpdatePrice(currentUser string, req types.AcctPriceCreateReq, id int64) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) DeletePrice(currentUser string, id int64) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) CreateOrder(currentUser string, req types.AcctOrderCreateReq) (*database.AccountOrder, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) ListRechargeByUserIDAndTime(ctx context.Context, req types.AcctRechargeListReq) (interface{}, error) {
	return nil, nil
}

func (ac *accountingComponentImpl) RechargesIndex(ctx context.Context, req types.RechargesIndexReq) ([]*types.RechargeIndexResp, int, error) {
	return nil, 0, nil
}

func (ac *accountingComponentImpl) StatementsIndex(ctx context.Context, req types.ActStatementsReq) ([]types.AcctStatementsRes, int, error) {
	return nil, 0, nil
}

func (ac *accountingComponentImpl) WeeklyRecharges(ctx context.Context) error {
	return nil
}
