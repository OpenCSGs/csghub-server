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

type accountingComponentImpl struct {
	acctountingClient accounting.AccountingClient
	userStore         database.UserStore
	deployTaskStore   database.DeployTaskStore
}

type AccountingComponent interface {
	ListMeteringsByUserIDAndTime(ctx context.Context, req types.ACCT_STATEMENTS_REQ) (interface{}, error)
}

func NewAccountingComponent(config *config.Config) (AccountingComponent, error) {
	c, err := accounting.NewAccountingClient(config)
	if err != nil {
		return nil, err
	}
	return &accountingComponentImpl{
		acctountingClient: c,
		userStore:         database.NewUserStore(),
		deployTaskStore:   database.NewDeployTaskStore(),
	}, nil
}

func (ac *accountingComponentImpl) ListMeteringsByUserIDAndTime(ctx context.Context, req types.ACCT_STATEMENTS_REQ) (interface{}, error) {
	user, err := ac.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	if user.UUID != req.UserUUID {
		return nil, errors.New("invalid user")
	}
	return ac.acctountingClient.ListMeteringsByUserIDAndTime(req)
}
