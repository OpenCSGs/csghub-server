package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type accountingComponentImpl struct {
	accountingClient      accounting.AccountingClient
	userStore             database.UserStore
	orgStore              database.OrgStore
	namespaceStore        database.NamespaceStore
	memberStore           database.MemberStore
	deployTaskStore       database.DeployTaskStore
	userSvcClient         rpc.UserSvcClient
	notificationSvcClient rpc.NotificationSvcClient
	config                *config.Config
}

type AccountingComponent interface {
	QueryAllUsersBalance(ctx context.Context, per, page int) (interface{}, error)
	QueryBalanceByUserID(ctx context.Context, currentUser, UUID string) (interface{}, error)
	QueryBalanceByUserIDInternal(ctx context.Context, currentUser string) (*database.AccountUser, error)
	ListStatementByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (interface{}, error)
	ListBillsByUserIDAndDate(ctx context.Context, req types.ActStatementsReq) (interface{}, error)
	ListBillsDetailByUserID(ctx context.Context, req types.AcctBillsDetailReq) (interface{}, error)
	RechargeAccountingUser(ctx context.Context, UUID string, req types.RechargeReq) (interface{}, error)
	CreateOrUpdateQuota(currentUser string, req types.AcctQuotaReq) (interface{}, error)
	GetQuotaByID(currentUser string) (interface{}, error)
	CreateQuotaStatement(currentUser string, req types.AcctQuotaStatementReq) (interface{}, error)
	GetQuotaStatement(currentUser string, req types.AcctQuotaStatementReq) (interface{}, error)
	QueryPricesBySKUType(currentUser string, req types.AcctPriceListReq) (*database.PriceResp, error)
	QueryPricesBySkuTypeAndKinds(currentUser string, req types.AcctPriceListByKindsReq) (any, error)
	GetPriceByID(currentUser string, id int64) (interface{}, error)
	CreatePrice(currentUser string, req types.AcctPriceCreateReq) (interface{}, error)
	UpdatePrice(currentUser string, req types.AcctPriceCreateReq, id int64) (interface{}, error)
	DeletePrice(currentUser string, id int64) (interface{}, error)
	CreateOrder(currentUser string, req types.AcctOrderCreateReq) (*database.AccountOrder, error)
	ListMeteringsByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (interface{}, error)
	ListRecharge(ctx context.Context, req types.AcctRechargeListReq) (interface{}, error)
	RechargesIndex(ctx context.Context, req types.RechargesIndexReq) ([]*types.RechargeIndexResp, int, error)
	StatementsIndex(ctx context.Context, req types.ActStatementsReq) ([]types.AcctStatementsRes, int, error)
	ListPresents(ctx context.Context, req types.PresentsIndexReq) ([]*types.PresentIndexResp, int, error)
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
		orgStore:         database.NewOrgStore(),
		memberStore:      database.NewMemberStore(),
		deployTaskStore:  database.NewDeployTaskStore(),
		userSvcClient:    userRpcClient,
		notificationSvcClient: rpc.NewNotificationSvcHttpClient(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
			rpc.AuthWithApiKey(config.APIToken)),
		config:         config,
		namespaceStore: database.NewNamespaceStore(),
	}, nil
}

func (ac *accountingComponentImpl) ListMeteringsByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (interface{}, error) {
	if err := ac.allowQueryData(ctx, req.CurrentUser, req.UserUUID); err != nil {
		return nil, errorx.Forbidden(err, map[string]any{
			"user": req.CurrentUser,
		})
	}
	return ac.accountingClient.ListMeteringsByUserIDAndTime(req)
}

// CanQueryUserData checks if the current user has permission to query the target user's data.
// Permission is granted if:
// 1. Current user is the same as the target user (querying own data)
// 2. Current user is an admin
// 3. Current user is a member of the organization that owns the target user's namespace
func (ac *accountingComponentImpl) allowQueryData(ctx context.Context, currentUser, targetUUID string) error {
	user, err := ac.userSvcClient.GetUserByName(ctx, currentUser)
	if err != nil {
		return fmt.Errorf("current user not found: %w", err)
	}

	if user.IsAdmin() || user.UUID == targetUUID {
		return nil
	}

	ns, err := ac.userSvcClient.GetNameSpaceInfoByUUID(ctx, targetUUID)
	if err != nil {
		return fmt.Errorf("target namespace not found: %w", err)
	}

	if ns.NSType != string(database.OrgNamespace) {
		return fmt.Errorf("do not have permission to query the target org's data: %w", err)
	}

	// Check if current user is member of org that owns target user's namespace
	role, err := ac.userSvcClient.GetMemberRoleByUUID(ctx, ns.UUID, currentUser)
	if err != nil || role == membership.RoleUnknown {
		return fmt.Errorf("do not have permission to query the target org's data: %w", err)
	}

	return nil
}
