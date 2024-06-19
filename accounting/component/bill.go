package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
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

func (a *AccountingBillComponent) ListBillsByUserIDAndDate(ctx context.Context, userID, startDate, endDate string) ([]database.AccountBill, error) {
	return a.abs.ListByUserIDAndDate(ctx, userID, startDate, endDate)
}
