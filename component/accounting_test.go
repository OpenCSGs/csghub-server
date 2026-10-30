package component

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockaccounting "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/accounting"
	mockrebac "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rebac"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// TestCheckOwnerOrOrgMemberPermissionAdminAuthorization verifies request grants and legacy administrator access.
func TestCheckOwnerOrOrgMemberPermissionAdminAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name       string
		authorized bool
		superAdmin bool
	}{
		{name: "authorized module administrator", authorized: true},
		{name: "legacy super administrator", superAdmin: true},
		{name: "ordinary user"},
	} {
		for _, namespaceType := range []database.NamespaceType{database.UserNamespace, database.OrgNamespace} {
			t.Run(tc.name+"/"+string(namespaceType), func(t *testing.T) {
				ctx := context.Background()
				if tc.authorized {
					ctx = types.WithAdminAuthorization(ctx)
				}
				ac := initializeTestAccountingComponent(ctx, t)
				ac.userSvcClient = ac.mocks.userSvcClient
				namespace := &rpc.Namespace{UUID: "target-uuid", NSType: string(namespaceType)}
				ac.mocks.userSvcClient.EXPECT().GetUserByName(ctx, "user").Return(&types.User{UUID: "user-uuid", SuperAdmin: tc.superAdmin}, nil).Once()
				ac.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, namespace.UUID).Return(namespace, nil).Once()
				if !tc.authorized && !tc.superAdmin && namespaceType == database.OrgNamespace {
					ac.rebac.(*mockrebac.MockAuthorizer).EXPECT().Check(ctx, rebac.CheckRequest{
						Subject: rebac.UserSubject("user-uuid"), Relation: rebac.NamespaceCanRead,
						Object: rebac.NamespaceObject(namespace.UUID), Consistency: rebac.ConsistencyHigher,
					}).Return(rebac.Decision{Allowed: false}, nil).Once()
				}

				result, err := checkOwnerOrOrgMemberPermission(ctx, ac.userSvcClient, ac.rebac, "user", namespace.UUID)
				require.Equal(t, namespace, result)
				if tc.authorized || tc.superAdmin {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			})
		}
	}
}

// TestCheckOwnerOrOrgMemberPermissionAdminAuthorizationRequiresTarget preserves target existence validation.
func TestCheckOwnerOrOrgMemberPermissionAdminAuthorizationRequiresTarget(t *testing.T) {
	ctx := types.WithAdminAuthorization(context.Background())
	ac := initializeTestAccountingComponent(ctx, t)
	ac.userSvcClient = ac.mocks.userSvcClient
	ac.mocks.userSvcClient.EXPECT().GetUserByName(ctx, "user").Return(&types.User{UUID: "user-uuid"}, nil).Once()
	ac.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, "missing-uuid").Return(nil, errors.New("not found")).Once()

	result, err := checkOwnerOrOrgMemberPermission(ctx, ac.userSvcClient, ac.rebac, "user", "missing-uuid")
	require.ErrorContains(t, err, "target namespace not found")
	require.Nil(t, result)
}

func TestAccountingComponent_ListMeteringsByUserIDAndTime(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)
	ac.userSvcClient = ac.mocks.userSvcClient

	req := types.ActStatementsReq{
		CurrentUser: "user",
		UserUUID:    "uuid",
	}
	ac.mocks.userSvcClient.EXPECT().GetUserByName(ctx, "user").Return(&types.User{
		UUID:  "uuid",
		Roles: []string{},
	}, nil)
	ac.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, "uuid").Return(&rpc.Namespace{
		UUID:   "uuid",
		NSType: string(database.UserNamespace),
	}, nil)
	ac.mocks.accountingClient.EXPECT().ListMeteringsByUserIDAndTime(req).Return(
		"", nil,
	)
	resp, err := ac.ListMeteringsByUserIDAndTime(ctx, req)
	require.Nil(t, err)
	require.Equal(t, "", resp)
}

// TestAccountingComponent_AdminUserTargetBoundary verifies both privileged methods reject non-user targets before billing calls.
func TestAccountingComponent_AdminUserTargetBoundary(t *testing.T) {
	lookupErr := errors.New("user database unavailable")
	for _, tc := range []struct {
		name, targetUUID string
		user             *database.User
		lookupErr        error
		wantErr          error
		noAuthorization  bool
		globalAdmin      bool
	}{
		{name: "missing authorization", targetUUID: "target-user", noAuthorization: true, wantErr: errorx.ErrForbidden},
		{name: "empty UUID", wantErr: errorx.ErrForbidden},
		{name: "organization UUID", targetUUID: "organization-uuid", lookupErr: errorx.ErrDatabaseNoRows, wantErr: errorx.ErrForbidden},
		{name: "global administrator organization UUID", targetUUID: "organization-uuid", globalAdmin: true, lookupErr: errorx.ErrDatabaseNoRows, wantErr: errorx.ErrForbidden},
		{name: "missing user", targetUUID: "missing-uuid", lookupErr: errorx.ErrDatabaseNoRows, wantErr: errorx.ErrForbidden},
		{name: "wrapped missing user", targetUUID: "missing-uuid", lookupErr: fmt.Errorf("lookup: %w", errorx.ErrDatabaseNoRows), wantErr: errorx.ErrForbidden},
		{name: "nil user", targetUUID: "target-user", wantErr: errorx.ErrForbidden},
		{name: "empty user", targetUUID: "target-user", user: &database.User{}, wantErr: errorx.ErrForbidden},
		{name: "mismatched UUID", targetUUID: "target-user", user: &database.User{UUID: "another-user"}, wantErr: errorx.ErrForbidden},
		{name: "deleted user", targetUUID: "target-user", user: &database.User{UUID: "target-user", DeletedAt: time.Now()}, wantErr: errorx.ErrForbidden},
		{name: "database failure", targetUUID: "target-user", lookupErr: lookupErr, wantErr: lookupErr},
	} {
		for _, operation := range []string{"balance", "recharge"} {
			t.Run(tc.name+"/"+operation, func(t *testing.T) {
				ctx := context.Background()
				if !tc.noAuthorization {
					ctx = types.WithAdminAuthorization(ctx)
				}
				users, billing, userRPC := mockdb.NewMockUserStore(t), mockaccounting.NewMockAccountingClient(t), mockrpc.NewMockUserSvcClient(t)
				ac := &accountingComponentImpl{userStore: users, accountingClient: billing, userSvcClient: userRPC}
				if !tc.noAuthorization && tc.targetUUID != "" {
					users.EXPECT().FindByUUID(ctx, tc.targetUUID).Return(tc.user, tc.lookupErr).Once()
				}
				if tc.globalAdmin {
					// Even a real global administrator cannot turn the user-only endpoint into organization billing.
					users.On("FindByUsername", ctx, "operator").Return(database.User{UUID: "operator-uuid", RoleAssignments: superAdminRoleAssignments()}, nil).Maybe()
					userRPC.On("GetUserByName", ctx, "operator").Return(&types.User{UUID: "operator-uuid", Admin: true, SuperAdmin: true}, nil).Maybe()
				}
				var response interface{}
				var err error
				if operation == "balance" {
					response, err = ac.QueryUserBalanceForAdmin(ctx, "operator", tc.targetUUID)
				} else {
					response, err = ac.RechargeUserForAdmin(ctx, tc.targetUUID, types.RechargeReq{Value: 100, CurrentUser: "operator", OpUID: "forged"})
				}
				require.Nil(t, response)
				require.ErrorIs(t, err, tc.wantErr)
				if errors.Is(tc.wantErr, lookupErr) {
					require.NotErrorIs(t, err, errorx.ErrForbidden)
				}
				billing.AssertNotCalled(t, "QueryBalanceByUserID", mock.Anything)
				billing.AssertNotCalled(t, "RechargeAccountingUser", mock.Anything, mock.Anything)
				userRPC.AssertNotCalled(t, "GetUserByName", mock.Anything, mock.Anything)
				userRPC.AssertNotCalled(t, "GetNameSpaceInfoByUUID", mock.Anything, mock.Anything)
				users.AssertNotCalled(t, "FindByUsername", mock.Anything, mock.Anything)
				if tc.noAuthorization || tc.targetUUID == "" {
					users.AssertNotCalled(t, "FindByUUID", mock.Anything, mock.Anything)
				}
			})
		}
	}
}

// TestAccountingComponent_AdminUserTargetAccepted keeps personal-user validation independent of role names or target metadata.
func TestAccountingComponent_AdminUserTargetAccepted(t *testing.T) {
	ctx := types.WithAdminAuthorization(context.Background())
	users := mockdb.NewMockUserStore(t)
	users.EXPECT().FindByUUID(ctx, "target-user").Return(&database.User{UUID: "target-user"}, nil).Once()
	ac := &accountingComponentImpl{userStore: users}
	require.NoError(t, ac.requireAdminUserTarget(ctx, "target-user"))
}
