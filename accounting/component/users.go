package component

import (
	"context"
	"database/sql"

	"opencsg.com/csghub-server/accounting/types"
	"opencsg.com/csghub-server/builder/store/database"
)

type AccountingUserComponent struct {
	au *database.AccountUserStore
}

func NewAccountingUser() *AccountingUserComponent {
	auc := &AccountingUserComponent{
		au: database.NewAccountUserStore(),
	}
	return auc
}

func (a *AccountingUserComponent) ListAccountingUser(ctx context.Context) ([]database.AccountUser, error) {
	return a.au.List(ctx)
}

func (a *AccountingUserComponent) GetAccountingByUserID(ctx context.Context, userID string) (*database.AccountUser, error) {
	return a.au.FindUserByID(ctx, userID)
}

func (a *AccountingUserComponent) AddNewAccountingUser(ctx context.Context, event *types.ACC_EVENT) error {
	statement := database.AccountUser{
		UserID:  event.UserID,
		Balance: 0,
	}
	return a.au.Create(ctx, statement)
}

func (a *AccountingUserComponent) CheckAccountingUser(ctx context.Context, userID string) error {
	_, err := a.au.FindUserByID(ctx, userID)
	if err == sql.ErrNoRows {
		statement := database.AccountUser{
			UserID:  userID,
			Balance: 0,
		}
		return a.au.Create(ctx, statement)
	}
	return err
}
