//go:build saas

package component

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountingComponent_QueryAllUsersBalance(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)
	ac.mocks.accountingClient.EXPECT().QueryAllUsersBalance(10, 1).Return(123, nil)
	data, err := ac.QueryAllUsersBalance(ctx, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 123, data)
}

func TestAccountingComponent_QueryBalanceByUserID(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	ac.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	ac.mocks.accountingClient.EXPECT().QueryBalanceByUserID("uuid").Return(123, nil)
	data, err := ac.QueryBalanceByUserID(ctx, "user", "uuid")
	require.Nil(t, err)
	require.Equal(t, 123, data)
}

func TestAccountingComponent_QueryBalanceByUserIDInternal(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	ac.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
		UUID:     "uuid",
	}, nil)
	ac.mocks.accountingClient.EXPECT().QueryBalanceByUserID("uuid").Return(map[string]any{
		"ID": 321,
	}, nil)
	data, err := ac.QueryBalanceByUserIDInternal(ctx, "user")
	require.Nil(t, err)
	require.Equal(t, &database.AccountUser{ID: 321}, data)
}

func TestAccountingComponent_ListStatementByUserIDAndTime(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	req := types.ActStatementsReq{CurrentUser: "user", UserUUID: "uuid"}
	ac.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
		UUID:     "uuid",
	}, nil)
	ac.mocks.accountingClient.EXPECT().ListStatementByUserIDAndTime(req).Return(123, nil)
	resp, err := ac.ListStatementByUserIDAndTime(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 123, resp)

}

func TestAccountingComponent_ListBillsByUserIDAndDate(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	req := types.ActStatementsReq{CurrentUser: "user", UserUUID: "uuid"}
	ac.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
		UUID:     "uuid",
	}, nil)
	rawBills := types.BILLS{
		AcctSummary: types.AcctSummary{Total: 100},
		Data: []types.ITEM{
			{Consumption: 1, InstanceName: "svc1", Value: 11},
			{Consumption: 2, InstanceName: "svc2", Value: 12},
		},
	}
	j, err := json.Marshal(rawBills)
	require.Nil(t, err)
	var tmp map[string]any
	err = json.Unmarshal(j, &tmp)
	require.Nil(t, err)
	ac.mocks.accountingClient.EXPECT().ListBillsByUserIDAndDate(req).Return(tmp, nil)
	ac.mocks.stores.DeployTaskMock().EXPECT().GetDeployBySvcName(ctx, "svc1").Return(
		&database.Deploy{ID: 1, DeployName: "d1", GitPath: "models_user/bar"}, nil,
	)
	ac.mocks.stores.DeployTaskMock().EXPECT().GetDeployBySvcName(ctx, "svc2").Return(
		&database.Deploy{ID: 2, DeployName: "d2", GitPath: "user/my_foo"}, nil,
	)
	resp, err := ac.ListBillsByUserIDAndDate(ctx, req)
	require.Nil(t, err)
	require.Equal(t, types.BILLS{
		Data: []types.ITEM{
			{
				InstanceName: "svc1", Consumption: 1, Value: 11,
				DeployID: 1, DeployName: "d1", Status: "Stopped", DeployUser: "user",
				RepoPath: "user/bar",
			},
			{
				InstanceName: "svc2", Consumption: 2, Value: 12,
				DeployID: 2, DeployName: "d2", Status: "Stopped", DeployUser: "user",
				RepoPath: "user/my_foo",
			},
		},
		AcctSummary: types.AcctSummary{
			Total: 100,
		},
	}, resp)

}

func TestAccountingComponent_RechargeAccountingUser(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	req := types.RechargeReq{
		Value: 123,
	}
	ac.mocks.stores.UserMock().EXPECT().FindByUUID(ctx, "uuid").Return(&database.User{}, nil)
	ac.mocks.accountingClient.EXPECT().RechargeAccountingUser("uuid", req).Return(1, nil)

	resp, err := ac.RechargeAccountingUser(ctx, "uuid", req)
	require.Nil(t, err)
	require.Equal(t, 1, resp)
}

func TestAccountingComponent_CreateOrUpdateQuota(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	req := types.AcctQuotaReq{}
	ac.mocks.accountingClient.EXPECT().CreateOrUpdateQuota("user", req).Return(1, nil)
	resp, err := ac.CreateOrUpdateQuota("user", req)
	require.Nil(t, err)
	require.Equal(t, 1, resp)
}

func TestAccountingComponent_GetQuotaByID(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	ac.mocks.accountingClient.EXPECT().GetQuotaByID("user").Return(1, nil)
	resp, err := ac.GetQuotaByID("user")
	require.Nil(t, err)
	require.Equal(t, 1, resp)
}

func TestAccountingComponent_CreateQuotaStatement(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	req := types.AcctQuotaStatementReq{}
	ac.mocks.accountingClient.EXPECT().CreateQuotaStatement("user", req).Return(1, nil)
	resp, err := ac.CreateQuotaStatement("user", req)
	require.Nil(t, err)
	require.Equal(t, 1, resp)
}

func TestAccountingComponent_GetQuotaStatement(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	req := types.AcctQuotaStatementReq{}
	ac.mocks.accountingClient.EXPECT().GetQuotaStatement("user", req).Return(1, nil)
	resp, err := ac.GetQuotaStatement("user", req)
	require.Nil(t, err)
	require.Equal(t, 1, resp)
}

func TestAccountingComponent_QueryPricesBySKUType(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	req := types.AcctPriceListReq{}
	ac.mocks.accountingClient.EXPECT().QueryPricesBySKUType("user", req).Return(
		map[string]any{"Total": 100}, nil,
	)
	resp, err := ac.QueryPricesBySKUType("user", req)
	require.Nil(t, err)
	require.Equal(t, &database.PriceResp{Total: 100}, resp)
}

func TestAccountingComponent_GetPriceByID(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	ac.mocks.accountingClient.EXPECT().GetPriceByID("user", int64(1)).Return(
		"", nil,
	)
	resp, err := ac.GetPriceByID("user", 1)
	require.Nil(t, err)
	require.Equal(t, "", resp)
}

func TestAccountingComponent_CreatePrice(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	req := types.AcctPriceCreateReq{}
	ac.mocks.accountingClient.EXPECT().CreatePrice("user", req).Return(
		"", nil,
	)
	resp, err := ac.CreatePrice("user", req)
	require.Nil(t, err)
	require.Equal(t, "", resp)
}

func TestAccountingComponent_UpdatePrice(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	req := types.AcctPriceCreateReq{}
	ac.mocks.accountingClient.EXPECT().UpdatePrice("user", req, int64(123)).Return(
		"", nil,
	)
	resp, err := ac.UpdatePrice("user", req, 123)
	require.Nil(t, err)
	require.Equal(t, "", resp)
}

func TestAccountingComponent_DeletePrice(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	ac.mocks.accountingClient.EXPECT().DeletePrice("user", int64(1)).Return(
		"", nil,
	)
	resp, err := ac.DeletePrice("user", 1)
	require.Nil(t, err)
	require.Equal(t, "", resp)
}

func TestAccountingComponent_CreateOrder(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	req := types.AcctOrderCreateReq{}
	ac.mocks.accountingClient.EXPECT().CreateOrder("user", req).Return(
		map[string]any{"order_uuid": "uuid"}, nil,
	)
	resp, err := ac.CreateOrder("user", req)
	require.Nil(t, err)
	require.Equal(t, &database.AccountOrder{OrderUUID: "uuid"}, resp)
}

func TestAccountingComponent_ListRechargeByUserIDAndTime(t *testing.T) {
	ctx := context.TODO()
	ac := initializeTestAccountingComponent(ctx, t)

	req := types.AcctRechargeListReq{CurrentUser: "user", UserUUID: "uuid"}
	ac.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
		UUID:     "uuid",
	}, nil)

	ac.mocks.accountingClient.EXPECT().ListRechargeByUserIDAndTime(req).Return(123, nil)

	resp, err := ac.ListRechargeByUserIDAndTime(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 123, resp)
}

func TestAccountingComponent_fetchListRecharges(t *testing.T) {
	tests := []struct {
		name           string
		req            types.RechargesIndexReq
		mockResponse   interface{}
		mockError      error
		expectedError  string
		expectedResult *types.RechargesIndexResp
	}{
		{
			name: "success - valid response",
			req: types.RechargesIndexReq{
				StartDate: "2024-01-01",
				EndDate:   "2024-01-07",
				Status:    "succeed",
				Per:       100,
				Page:      1,
			},
			mockResponse: &types.RechargesIndexResp{
				Data: []*types.RechargeIndexResp{
					{
						RechargeResp: types.RechargeResp{
							RechargeUUID:   "uuid1",
							UserUUID:       "user1",
							Amount:         10000,
							PaymentChannel: "wx_pub_qr",
							CreatedAt:      time.Now(),
						},
						Amount: 100.0,
					},
				},
				Total: 1,
				Sum:   10000,
			},
			mockError:     nil,
			expectedError: "",
			expectedResult: &types.RechargesIndexResp{
				Data: []*types.RechargeIndexResp{
					{
						RechargeResp: types.RechargeResp{
							RechargeUUID:   "uuid1",
							UserUUID:       "user1",
							Amount:         10000,
							PaymentChannel: "wx_pub_qr",
							CreatedAt:      time.Now(),
						},
						Amount: 100.0,
					},
				},
				Total: 1,
				Sum:   10000,
			},
		},
		{
			name: "error - ListRecharges returns error",
			req: types.RechargesIndexReq{
				StartDate: "2024-01-01",
				EndDate:   "2024-01-07",
				Status:    "succeed",
				Per:       100,
				Page:      1,
			},
			mockResponse:  nil,
			mockError:     fmt.Errorf("network error"),
			expectedError: "list recharges error",
		},
		{
			name: "success - empty response",
			req: types.RechargesIndexReq{
				StartDate: "2024-01-01",
				EndDate:   "2024-01-07",
				Status:    "succeed",
				Per:       100,
				Page:      1,
			},
			mockResponse: &types.RechargesIndexResp{
				Data:  []*types.RechargeIndexResp{},
				Total: 0,
				Sum:   0,
			},
			mockError:     nil,
			expectedError: "",
			expectedResult: &types.RechargesIndexResp{
				Data:  []*types.RechargeIndexResp{},
				Total: 0,
				Sum:   0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			ac := initializeTestAccountingComponent(ctx, t)

			// Convert expected response to map[string]any for mock (matching the pattern in other tests)
			var mockReturn interface{}
			if tt.mockResponse != nil {
				jsonBytes, err := json.Marshal(tt.mockResponse)
				require.Nil(t, err)
				var tmp map[string]any
				err = json.Unmarshal(jsonBytes, &tmp)
				require.Nil(t, err)
				mockReturn = tmp
			}

			ac.mocks.accountingClient.EXPECT().ListRecharges(tt.req).Return(mockReturn, tt.mockError)

			result, err := ac.fetchListRecharges(tt.req)

			if tt.expectedError != "" {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
				require.Nil(t, result)
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				require.Equal(t, tt.expectedResult.Total, result.Total)
				require.Equal(t, tt.expectedResult.Sum, result.Sum)
				require.Equal(t, len(tt.expectedResult.Data), len(result.Data))
				if len(tt.expectedResult.Data) > 0 {
					require.Equal(t, tt.expectedResult.Data[0].RechargeUUID, result.Data[0].RechargeUUID)
					require.Equal(t, tt.expectedResult.Data[0].Amount, result.Data[0].Amount)
				}
			}
		})
	}
}
