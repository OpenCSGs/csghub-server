package component

import (
	"context"
	"errors"
	"fmt"

	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type AccountingComponent struct {
	acctClient *accounting.AccountingClient
	user       *database.UserStore
	deploy     *database.DeployTaskStore
}

func NewAccountingComponent(config *config.Config) (*AccountingComponent, error) {
	c, err := accounting.NewAccountingClient(config)
	if err != nil {
		return nil, err
	}
	return &AccountingComponent{
		acctClient: c,
		user:       database.NewUserStore(),
		deploy:     database.NewDeployTaskStore(),
	}, nil
}

func (ac *AccountingComponent) ListMeteringsByUserIDAndTime(ctx context.Context, req types.ACCT_STATEMENTS_REQ) (interface{}, error) {
	user, err := ac.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	if user.UUID != req.UserUUID {
		return nil, errors.New("invalid user")
	}
	return ac.acctClient.ListMeteringsByUserIDAndTime(req)
}
