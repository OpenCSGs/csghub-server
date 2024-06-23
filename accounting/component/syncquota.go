package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"opencsg.com/csghub-server/accounting/types"
	"opencsg.com/csghub-server/builder/store/database"
)

type AccountingSyncQuotaComponent struct {
	asq  *database.AccountSyncQuotaStore
	user *database.UserStore
}

func NewAccountingQuotaComponent() *AccountingSyncQuotaComponent {
	asqc := &AccountingSyncQuotaComponent{
		asq:  database.NewAccountSyncQuotaStore(),
		user: database.NewUserStore(),
	}
	return asqc
}

func (a *AccountingSyncQuotaComponent) GetQuotaByID(ctx context.Context, currentUser string) (*database.AccountSyncQuota, error) {
	user, err := a.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	acctQuota, err := a.asq.GetByID(ctx, user.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fail to get account quota, %w", err)
	}
	return acctQuota, nil
}

func (a *AccountingSyncQuotaComponent) CreateOrUpdateQuota(ctx context.Context, currentUser string, input types.ACCT_QUOTA_REQ) (*database.AccountSyncQuota, error) {
	user, err := a.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	quota, err := a.asq.GetByID(ctx, user.ID)
	if errors.Is(err, sql.ErrNoRows) {
		// create account quota
		acctQuota := database.AccountSyncQuota{
			UserID:         user.ID,
			RepoCountLimit: input.RepoCountLimit,
			RepoCountUsed:  0,
			SpeedLimit:     input.SpeedLimit,
			TrafficLimit:   input.TrafficLimit,
			TrafficUsed:    0,
		}
		err := a.asq.Create(ctx, acctQuota)
		if err != nil {
			return nil, fmt.Errorf("fail to create account quota, %w", err)
		}
		return &acctQuota, nil
	}

	if err != nil || quota == nil {
		return nil, fmt.Errorf("fail to query account quota, %w", err)
	}

	// update account quota
	quota.RepoCountLimit = input.RepoCountLimit
	quota.SpeedLimit = input.SpeedLimit
	quota.TrafficLimit = input.TrafficLimit
	_, err = a.asq.Update(ctx, *quota)
	if err != nil {
		return nil, fmt.Errorf("fail to update account quota, %w", err)
	}

	return quota, nil
}
