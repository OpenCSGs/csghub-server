package component

import (
	"context"
	"database/sql"
	"errors"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
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

func (a *AccountingUserComponent) ListAccountingUser(ctx context.Context, per, page int) ([]database.AccountUser, int, error) {
	return a.au.List(ctx, per, page)
}

func (a *AccountingUserComponent) GetAccountingByUserID(ctx context.Context, userID string) (*database.AccountUser, error) {
	account, err := a.au.FindUserByID(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return &database.AccountUser{
			UserID:  userID,
			Balance: 0,
		}, nil
	}
	return account, err
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
	if errors.Is(err, sql.ErrNoRows) {
		statement := database.AccountUser{
			UserID:  userID,
			Balance: 0,
		}
		return a.au.Create(ctx, statement)
	}
	return err
}
