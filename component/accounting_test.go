package component

import (
	"context"
	"errors"
	"testing"

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

// TestAccountingComponent_AdminUserTargetBoundary verifies both privileged methods reject invalid targets before billing calls.
func TestAccountingComponent_AdminUserTargetBoundary(t *testing.T) {
	lookupErr := errors.New("namespace service unavailable")
	for _, tc := range []struct {
		name, targetUUID string
		namespace        *rpc.Namespace
		lookupErr        error
		wantErr          error
		wantContains     string
		noAuthorization  bool
	}{
		{name: "missing authorization", targetUUID: "target-user", noAuthorization: true, wantErr: errorx.ErrForbidden},
		{name: "empty UUID", wantErr: errorx.ErrForbidden},
		{name: "nil namespace", targetUUID: "target-user", wantErr: errorx.ErrForbidden},
		{name: "namespace lookup failure", targetUUID: "target-user", lookupErr: lookupErr, wantContains: "target namespace not found"},
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
					userRPC.EXPECT().GetNameSpaceInfoByUUID(ctx, tc.targetUUID).Return(tc.namespace, tc.lookupErr).Once()
				}
				var response interface{}
				var err error
				if operation == "balance" {
					response, err = ac.QueryUserBalanceForAdmin(ctx, "operator", tc.targetUUID)
				} else {
					response, err = ac.RechargeUserForAdmin(ctx, tc.targetUUID, types.RechargeReq{Value: 100, CurrentUser: "operator", OpUID: "forged"})
				}
				require.Nil(t, response)
				if tc.wantErr != nil {
					require.ErrorIs(t, err, tc.wantErr)
				}
				if tc.wantContains != "" {
					require.ErrorContains(t, err, tc.wantContains)
					require.ErrorIs(t, err, lookupErr)
					require.NotErrorIs(t, err, errorx.ErrForbidden)
				}
				billing.AssertNotCalled(t, "QueryBalanceByUserID", mock.Anything)
				billing.AssertNotCalled(t, "RechargeAccountingUser", mock.Anything, mock.Anything)
				userRPC.AssertNotCalled(t, "GetUserByName", mock.Anything, mock.Anything)
				users.AssertNotCalled(t, "FindByUUID", mock.Anything, mock.Anything)
				users.AssertNotCalled(t, "FindByUsername", mock.Anything, mock.Anything)
				if tc.noAuthorization || tc.targetUUID == "" {
					userRPC.AssertNotCalled(t, "GetNameSpaceInfoByUUID", mock.Anything, mock.Anything)
				}
			})
		}
	}
}

// TestAccountingComponent_AdminUserTargetAccepted keeps target validation independent of role names or namespace type.
func TestAccountingComponent_AdminUserTargetAccepted(t *testing.T) {
	for _, namespaceType := range []database.NamespaceType{database.UserNamespace, database.OrgNamespace} {
		t.Run(string(namespaceType), func(t *testing.T) {
			ctx := types.WithAdminAuthorization(context.Background())
			userRPC := mockrpc.NewMockUserSvcClient(t)
			userRPC.EXPECT().GetNameSpaceInfoByUUID(ctx, "target-uuid").Return(&rpc.Namespace{
				UUID:   "target-uuid",
				NSType: string(namespaceType),
			}, nil).Once()
			ac := &accountingComponentImpl{userSvcClient: userRPC}
			require.NoError(t, ac.requireAdminUserTarget(ctx, "target-uuid"))
		})
	}
}
