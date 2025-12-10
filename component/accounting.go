package component

import (
	"context"
	"errors"
	"fmt"

	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type accountingComponentImpl struct {
	accountingClient      accounting.AccountingClient
	userStore             database.UserStore
	deployTaskStore       database.DeployTaskStore
	userSvcClient         rpc.UserSvcClient
	notificationSvcClient rpc.NotificationSvcClient
	config                *config.Config
}

type AccountingComponent interface {
	QueryAllUsersBalance(ctx context.Context, per, page int) (interface{}, error)
	QueryBalanceByUserID(ctx context.Context, currentUser, userUUID string) (interface{}, error)
	QueryBalanceByUserIDInternal(ctx context.Context, currentUser string) (*database.AccountUser, error)
	ListStatementByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (interface{}, error)
	ListBillsByUserIDAndDate(ctx context.Context, req types.ActStatementsReq) (interface{}, error)
	RechargeAccountingUser(ctx context.Context, userUUID string, req types.RechargeReq) (interface{}, error)
	CreateOrUpdateQuota(currentUser string, req types.AcctQuotaReq) (interface{}, error)
	GetQuotaByID(currentUser string) (interface{}, error)
	CreateQuotaStatement(currentUser string, req types.AcctQuotaStatementReq) (interface{}, error)
	GetQuotaStatement(currentUser string, req types.AcctQuotaStatementReq) (interface{}, error)
	QueryPricesBySKUType(currentUser string, req types.AcctPriceListReq) (*database.PriceResp, error)
	GetPriceByID(currentUser string, id int64) (interface{}, error)
	CreatePrice(currentUser string, req types.AcctPriceCreateReq) (interface{}, error)
	UpdatePrice(currentUser string, req types.AcctPriceCreateReq, id int64) (interface{}, error)
	DeletePrice(currentUser string, id int64) (interface{}, error)
	CreateOrder(currentUser string, req types.AcctOrderCreateReq) (*database.AccountOrder, error)
	ListMeteringsByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (interface{}, error)
	ListRechargeByUserIDAndTime(ctx context.Context, req types.AcctRechargeListReq) (interface{}, error)
	RechargesIndex(ctx context.Context, req types.RechargesIndexReq) ([]*types.RechargeIndexResp, int, error)
	StatementsIndex(ctx context.Context, req types.ActStatementsReq) ([]types.AcctStatementsRes, int, error)
	WeeklyRecharges(ctx context.Context) error
}

func NewAccountingComponent(config *config.Config) (AccountingComponent, error) {
	c, err := accounting.NewAccountingClient(config)
	if err != nil {
		return nil, err
	}
	userSvcAddr := fmt.Sprintf("%s:%d", config.User.Host, config.User.Port)
	userRpcClient := rpc.NewUserSvcHttpClient(userSvcAddr, rpc.AuthWithApiKey(config.APIToken))
	return &accountingComponentImpl{
		accountingClient: c,
		userStore:        database.NewUserStore(),
		deployTaskStore:  database.NewDeployTaskStore(),
		userSvcClient:    userRpcClient,
		notificationSvcClient: rpc.NewNotificationSvcHttpClient(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
			rpc.AuthWithApiKey(config.APIToken)),
		config: config,
	}, nil
}

func (ac *accountingComponentImpl) ListMeteringsByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (interface{}, error) {
	user, err := ac.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, %w", err)
	}
	if user.UUID != req.UserUUID {
		return nil, errors.New("invalid user")
	}
	return ac.accountingClient.ListMeteringsByUserIDAndTime(req)
}
