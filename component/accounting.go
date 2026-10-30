package component

import (
	"context"
	"errors"
	"fmt"

	"opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/builder/rebac"
	rebacfactory "opencsg.com/csghub-server/builder/rebac/factory"
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
	rebac                 rebac.Authorizer
}

type AccountingComponent interface {
	QueryAllUsersBalance(ctx context.Context, per, page int) (interface{}, error)
	QueryBalanceByUserID(ctx context.Context, currentUser, UUID string) (interface{}, error)
	// QueryUserBalanceForAdmin confines an authorized account/user request to an active personal user.
	QueryUserBalanceForAdmin(ctx context.Context, currentUser, UUID string) (interface{}, error)
	QueryBalanceByUserIDInternal(ctx context.Context, currentUser string) (*types.UserBalanceResp, error)
	ListStatementByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (interface{}, error)
	ListBillsByUserIDAndDate(ctx context.Context, req types.ActStatementsReq) (interface{}, error)
	ListBillsDetailByUserID(ctx context.Context, req types.AcctBillsDetailReq) (interface{}, error)
	RechargeAccountingUser(ctx context.Context, UUID string, req types.RechargeReq) (interface{}, error)
	// RechargeUserForAdmin confines an authorized account/user recharge to an active personal user.
	RechargeUserForAdmin(ctx context.Context, UUID string, req types.RechargeReq) (interface{}, error)
	CreateOrUpdateQuota(currentUser string, req types.AcctQuotaReq) (interface{}, error)
	GetQuotaByID(currentUser string) (interface{}, error)
	CreateQuotaStatement(currentUser string, req types.AcctQuotaStatementReq) (interface{}, error)
	GetQuotaStatement(currentUser string, req types.AcctQuotaStatementReq) (interface{}, error)
	QueryPricesBySKUType(currentUser string, req types.AcctPriceListReq) (*database.PriceResp, error)
	QueryPricesBySkuTypeAndKinds(currentUser string, req types.AcctPriceListByKindsReq) (any, error)
	QueryDistinctPrices(currentUser string, req types.AcctPriceDistinctListReq) (any, error)
	GetPriceByID(currentUser string, id int64) (interface{}, error)
	CreatePrice(currentUser string, req types.AcctPriceCreateReq) (interface{}, error)
	BatchCreatePrice(currentUser string, req types.AcctPriceBatchCreateReq) (interface{}, error)
	UpdatePrice(currentUser string, req types.AcctPriceUpdateReq, id int64) (interface{}, error)
	DeletePrice(currentUser string, id int64) (interface{}, error)
	CreateOrder(currentUser string, req types.AcctOrderCreateReq) (*database.AccountOrder, error)
	ListMeteringsByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (interface{}, error)
	ListRecharge(ctx context.Context, req types.AcctRechargeListReq) (interface{}, error)
	RechargesIndex(ctx context.Context, req types.RechargesIndexReq) ([]*types.RechargeIndexResp, int, error)
	StatementsIndex(ctx context.Context, req types.ActStatementsReq) ([]types.AcctStatementsRes, int, error)
	TokenReportIndex(ctx context.Context, req types.TokenReportReq) (*types.TokenReportResp, error)
	ListPresents(ctx context.Context, req types.PresentsIndexReq) ([]*types.PresentIndexResp, int, error)
	WeeklyRecharges(ctx context.Context, emails []string) error
	GetOrderDetailByID(ctx context.Context, currentUser string, id int64) (*database.AccountOrderDetail, error)
	GetVoucherDashboard(ctx context.Context, req types.VoucherDashboardReq) (*types.VoucherDashboardStatusItem, error)
	OffLinePrice(ctx context.Context, req types.AcctPriceOffLineReq) (any, error)
	QueryStatisticsByUserID(ctx context.Context, req types.AcctBillsReq) (interface{}, error)
	QueryStatisticsDetailByUserID(ctx context.Context, req types.AcctBillsDetailReq) (interface{}, error)
	GetStatisticsSummary(ctx context.Context, req types.AcctBillsReq) (interface{}, error)
}

func NewAccountingComponent(config *config.Config) (AccountingComponent, error) {
	c, err := accounting.NewAccountingClient(config)
	if err != nil {
		return nil, err
	}
	userSvcAddr := fmt.Sprintf("%s:%d", config.User.Host, config.User.Port)
	userRpcClient := rpc.NewUserSvcHttpClient(userSvcAddr, rpc.AuthWithApiKey(config.APIToken))
	authorizer, err := rebacfactory.NewAuthorizer()
	if err != nil {
		return nil, fmt.Errorf("failed to create ReBAC authorizer: %w", err)
	}
	return &accountingComponentImpl{
		accountingClient: c,
		userStore:        database.NewUserStore(),
		orgStore:         database.NewOrgStore(config.IsHierarchicalOrganization(), nil),
		memberStore:      database.NewMemberStore(),
		deployTaskStore:  database.NewDeployTaskStore(),
		userSvcClient:    userRpcClient,
		notificationSvcClient: rpc.NewNotificationSvcHttpClient(fmt.Sprintf("%s:%d", config.Notification.Host, config.Notification.Port),
			rpc.AuthWithApiKey(config.APIToken)),
		config:         config,
		namespaceStore: database.NewNamespaceStore(),
		rebac:          authorizer,
	}, nil
}

// QueryUserBalanceForAdmin validates the user-only administration boundary before reusing balance retrieval.
func (ac *accountingComponentImpl) QueryUserBalanceForAdmin(ctx context.Context, currentUser, UUID string) (interface{}, error) {
	if err := ac.requireAdminUserTarget(ctx, UUID); err != nil {
		return nil, err
	}
	return ac.QueryBalanceByUserID(ctx, currentUser, UUID)
}

// RechargeUserForAdmin validates the user-only administration boundary before resolving the real recharge operator.
func (ac *accountingComponentImpl) RechargeUserForAdmin(ctx context.Context, UUID string, req types.RechargeReq) (interface{}, error) {
	if err := ac.requireAdminUserTarget(ctx, UUID); err != nil {
		return nil, err
	}
	return ac.RechargeAccountingUser(ctx, UUID, req)
}

// requireAdminUserTarget accepts only active personal user records for an already authorized administrator request.
// Global administrator permission does not expand this entry point to organization billing accounts.
func (ac *accountingComponentImpl) requireAdminUserTarget(ctx context.Context, targetUUID string) error {
	if !types.IsAdminAuthorized(ctx) || targetUUID == "" {
		return errorx.ErrForbidden
	}
	user, err := ac.userStore.FindByUUID(ctx, targetUUID)
	if errors.Is(err, errorx.ErrDatabaseNoRows) {
		return errorx.ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("find administrator accounting user target: %w", err)
	}
	if user == nil || user.UUID != targetUUID || !user.DeletedAt.IsZero() {
		return errorx.ErrForbidden
	}
	return nil
}

func (ac *accountingComponentImpl) ListMeteringsByUserIDAndTime(ctx context.Context, req types.ActStatementsReq) (interface{}, error) {
	if _, err := checkOwnerOrOrgMemberPermission(ctx, ac.userSvcClient, ac.rebac, req.CurrentUser, req.UserUUID); err != nil {
		return nil, errorx.Forbidden(err, map[string]any{
			"user": req.CurrentUser,
		})
	}
	return ac.accountingClient.ListMeteringsByUserIDAndTime(req)
}

// checkOwnerOrOrgMemberPermission checks permission to query the target user's data.
// Permission is granted if:
// 1. Current user is the same as the target user (querying own data)
// 2. The request has administrator authorization or the user has global super administrator permission
// 3. Current user can read the organization namespace that owns the target user's namespace
func checkOwnerOrOrgMemberPermission(ctx context.Context, userSvcClient rpc.UserSvcClient, authorizer rebac.Authorizer, currentUser, targetUUID string) (*rpc.Namespace, error) {
	user, err := userSvcClient.GetUserByName(ctx, currentUser)
	if err != nil {
		return nil, fmt.Errorf("current user not found: %w", err)
	}

	ns, err := userSvcClient.GetNameSpaceInfoByUUID(ctx, targetUUID)
	if err != nil {
		return ns, fmt.Errorf("target namespace not found: %w", err)
	}

	if types.IsAdminAuthorized(ctx) || user.IsSuperAdmin() || user.UUID == targetUUID {
		return ns, nil
	}

	if ns.NSType != string(database.OrgNamespace) {
		return ns, fmt.Errorf("do not have permission to query the target user's data")
	}

	if authorizer == nil {
		return ns, fmt.Errorf("namespace ReBAC authorizer is required")
	}

	// Check whether the current user can read the organization namespace.
	decision, err := authorizer.Check(ctx, rebac.CheckRequest{
		Subject:     rebac.UserSubject(user.UUID),
		Relation:    rebac.NamespaceCanRead,
		Object:      rebac.NamespaceObject(ns.UUID),
		Consistency: rebac.ConsistencyHigher,
	})
	if err != nil {
		return ns, fmt.Errorf("failed to check target organization namespace permission: %w", err)
	}
	if !decision.Allowed {
		return ns, fmt.Errorf("do not have permission to query the target org's data")
	}

	return ns, nil
}
