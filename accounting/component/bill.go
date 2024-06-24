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

func (a *AccountingBillComponent) ListBillsByUserIDAndDate(ctx context.Context, userUUID, startDate, endDate string, scene, per, page int) ([]map[string]interface{}, int, error) {
	return a.abs.ListByUserIDAndDate(ctx, userUUID, startDate, endDate, scene, per, page)
}
