package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type AccountingBillComponent struct {
	abs *database.AccountBillStore
}

func NewAccountingBill() *AccountingBillComponent {
	abc := &AccountingBillComponent{
		abs: database.NewAccountBillStore(),
	}
	return abc
}

func (a *AccountingBillComponent) ListBillsByUserIDAndDate(ctx context.Context, req types.ACCT_BILLS_REQ) (database.AccountBillRes, error) {
	return a.abs.ListByUserIDAndDate(ctx, req)
}
