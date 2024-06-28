package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

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

func (ac *AccountingComponent) QueryAllUsersBalance(ctx context.Context, currentUser string, per, page int) (interface{}, error) {
	_, err := ac.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	return ac.acctClient.QueryAllUsersBalance(per, page)
}

func (ac *AccountingComponent) QueryBalanceByUserID(ctx context.Context, currentUser, userUUID string) (interface{}, error) {
	user, err := ac.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	if user.UUID != userUUID {
		return nil, errors.New("invalid user")
	}
	return ac.acctClient.QueryBalanceByUserID(userUUID)
}

func (ac *AccountingComponent) QueryBalanceByUserIDInternal(ctx context.Context, currentUser string) (*database.AccountUser, error) {
	user, err := ac.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	resp, err := ac.acctClient.QueryBalanceByUserID(user.UUID)
	if err != nil {
		slog.Error("error to get user balance data", slog.Any("error", err))
		return nil, err
	}

	tempJSON, err := json.Marshal(resp)
	if err != nil {
		slog.Error("error to marshal json", slog.Any("error", err))
		return nil, err
	}
	var account *database.AccountUser
	if err := json.Unmarshal(tempJSON, &account); err != nil {
		slog.Error("error to unmarshal json", slog.Any("error", err))
		return nil, err
	}
	return account, nil
}

func (ac *AccountingComponent) ListStatementByUserIDAndTime(ctx context.Context, req types.ACCT_STATEMENTS_REQ) (interface{}, error) {
	user, err := ac.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	if user.UUID != req.UserUUID {
		return nil, errors.New("invalid user")
	}
	return ac.acctClient.ListStatementByUserIDAndTime(req)
}

func (ac *AccountingComponent) ListBillsByUserIDAndDate(ctx context.Context, req types.ACCT_STATEMENTS_REQ) (interface{}, error) {
	user, err := ac.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	if user.UUID != req.UserUUID {
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
	var mergedItems []types.ITEM
	for _, item := range bills.Data {
		newItem := types.ITEM{
			Consumption:  item.Consumption,
			InstanceName: item.InstanceName,
			Value:        item.Value,
		}
		d, _ := ac.deploy.GetDeployBySvcName(ctx, item.InstanceName)
		if d != nil {
			newItem.Status = deployStatusCodeToString(d.Status)
			newItem.CreatedAt = d.CreatedAt
			newItem.DeployID = d.ID
			if d.GitPath != "" {
				cols := strings.Split(d.GitPath, "_")
				if len(cols) > 1 {
					newItem.RepoPath = cols[1]
				}
			}
		}
		mergedItems = append(mergedItems, newItem)
	}
	return types.BILLS{
		Data: mergedItems,
		ACCT_SUMMARY: types.ACCT_SUMMARY{
			Total:            bills.Total,
			TotalValue:       bills.TotalValue,
			TotalConsumption: bills.TotalConsumption,
		},
	}, err
}

func (ac *AccountingComponent) RechargeAccountingUser(ctx context.Context, currentUser, userUUID string, req types.RECHARGE_REQ) (interface{}, error) {
	_, err := ac.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	_, err = ac.user.FindByUUID(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("invalid user uuid, %w", err)
	}
	// Todo: check super admin to do this action
	return ac.acctClient.RechargeAccountingUser(userUUID, req)
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

func (ac *AccountingComponent) parseBillsData(data interface{}) (*types.BILLS, error) {
	resByte, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var newData types.BILLS
	err = json.Unmarshal(resByte, &newData)
	if err != nil {
		return nil, err
	}
	return &newData, nil
}
