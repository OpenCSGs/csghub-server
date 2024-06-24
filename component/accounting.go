package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type Item struct {
	Consumption  float64   `json:"consumption"`
	InstanceName string    `json:"instance_name"`
	Value        float64   `json:"value"`
	CreatedAt    time.Time `json:"created_at"`
	Status       string    `json:"status"`
}
type Bills struct {
	Total int    `json:"total"`
	Data  []Item `json:"data"`
}

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

func (ac *AccountingComponent) QueryAllUsersBalance(ctx context.Context, currentUser string) (interface{}, error) {
	_, err := ac.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	return ac.acctClient.QueryAllUsersBalance()
}

func (ac *AccountingComponent) QueryBalanceByUserID(ctx context.Context, currentUser, userUUID string) (interface{}, error) {
	user, err := ac.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	if user.CasdoorUUID != userUUID {
		return nil, errors.New("invalid user")
	}
	return ac.acctClient.QueryBalanceByUserID(userUUID)
}

func (ac *AccountingComponent) ListStatementByUserIDAndTime(ctx context.Context, req types.ACCT_STATEMENTS_REQ) (interface{}, error) {
	user, err := ac.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	if user.CasdoorUUID != req.UserID {
		return nil, errors.New("invalid user")
	}
	return ac.acctClient.ListStatementByUserIDAndTime(req)
}

func (ac *AccountingComponent) ListBillsByUserIDAndDate(ctx context.Context, req types.ACCT_STATEMENTS_REQ) (interface{}, error) {
	user, err := ac.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	if user.CasdoorUUID != req.UserID {
		return nil, errors.New("invalid user")
	}

	data, err := ac.acctClient.ListBillsByUserIDAndDate(req)
	if err != nil {
		return nil, err
	}
	bills, err := ac.parseBillsData(data)
	// slog.Info("convert", slog.Any("data", data), slog.Any("bills", bills))
	if err != nil {
		return nil, err
	}
	if bills == nil || bills.Data == nil {
		return bills, nil
	}
	var mergedItems []Item
	for _, item := range bills.Data {
		newItem := Item{
			Consumption:  item.Consumption,
			InstanceName: item.InstanceName,
			Value:        item.Value,
		}
		d, err := ac.deploy.GetDeployBySvcName(ctx, item.InstanceName)
		if err == nil || d != nil {
			newItem.Status = deployStatusCodeToString(d.Status)
			newItem.CreatedAt = d.CreatedAt
		}
		mergedItems = append(mergedItems, newItem)
	}
	return Bills{Data: mergedItems, Total: bills.Total}, err
}

func (ac *AccountingComponent) RechargeAccountingUser(ctx context.Context, currentUser, userID string, req types.RECHARGE_REQ) (interface{}, error) {
	_, err := ac.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	_, err = ac.user.FindByCasdoorUUID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user casdoor uuid, %w", err)
	}
	// Todo: check super admin to do this action
	return ac.acctClient.RechargeAccountingUser(userID, req)
}

func (ac *AccountingComponent) CreateOrUpdateQuota(currentUser string, req types.ACCT_QUOTA_REQ) (interface{}, error) {
	return ac.acctClient.CreateOrUpdateQuota(currentUser, req)
}

func (ac *AccountingComponent) GetQuotaByID(currentUser string) (interface{}, error) {
	return ac.acctClient.GetQuotaByID(currentUser)
}

func (ac *AccountingComponent) CreateQuotaStatement(currentUser string, req types.ACCT_QUOTA_STATEMENT_REQ) (interface{}, error) {
	return ac.acctClient.CreateQuotaStatement(currentUser, req)
}

func (ac *AccountingComponent) GetQuotaStatement(currentUser string, req types.ACCT_QUOTA_STATEMENT_REQ) (interface{}, error) {
	return ac.acctClient.GetQuotaStatement(currentUser, req)
}

func (ac *AccountingComponent) parseBillsData(data interface{}) (*Bills, error) {
	resByte, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var newData Bills
	err = json.Unmarshal(resByte, &newData)
	if err != nil {
		return nil, err
	}
	return &newData, nil
}
