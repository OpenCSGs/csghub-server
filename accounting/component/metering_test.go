package component

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/accounting/utils"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func NewTestMeteringComponent(amss database.AccountMeteringStore) MeteringComponent {
	ams := &meteringComponentImpl{
		ams: amss,
	}
	return ams
}

func TestMeteringComponent_SaveMeteringEventRecord(t *testing.T) {
	ctx := context.TODO()

	uid := uuid.New()
	req := types.MeteringEvent{
		Uuid:       uid,
		UserUUID:   "test-user-uuid",
		Value:      100,
		ValueType:  10,
		Scene:      1,
		OpUID:      "test-op-uid",
		ResourceID: "test-ID",
	}

	extra := types.MeteringExtra{
		SkuUnitType: utils.GetSkuUnitTypeByScene(types.SceneType(req.Scene)),
	}

	data := database.AccountMetering{
		EventUUID:    req.Uuid,
		UserUUID:     req.UserUUID,
		Value:        float64(req.Value),
		ValueType:    req.ValueType,
		Scene:        types.SceneType(req.Scene),
		OpUID:        req.OpUID,
		ResourceID:   req.ResourceID,
		ResourceName: req.ResourceName,
		CustomerID:   req.CustomerID,
		RecordedAt:   req.CreatedAt,
		Extra:        req.Extra,
		SkuUnitType:  extra.SkuUnitType,
	}

	mockStore := mockdb.NewMockAccountMeteringStore(t)
	mockStore.EXPECT().Create(ctx, data, extra).Return(nil)

	mockComp := NewTestMeteringComponent(mockStore)

	err := mockComp.SaveMeteringEventRecord(ctx, &req, extra)

	require.Nil(t, err)
}

func TestMeteringComponent_ListMeteringByUserIDAndDate(t *testing.T) {
	ctx := context.TODO()

	req := types.ActStatementsReq{
		UserUUID:  "test-user-uuid",
		Scene:     types.SceneModelInference,
		StartTime: "2024-01-01",
		EndTime:   "2024-12-31",
		Per:       10,
		Page:      1,
	}

	data := []database.AccountMetering{
		{
			EventUUID: uuid.New(),
			UserUUID:  "test-user-uuid",
			Value:     100,
			ValueType: 10,
			Scene:     types.SceneType(types.SceneModelInference),
			OpUID:     "test-op-uid",
		},
	}

	mockStore := mockdb.NewMockAccountMeteringStore(t)
	mockStore.EXPECT().ListByUserIDAndTime(ctx, req).Return(data, 1, nil)

	mockComp := NewTestMeteringComponent(mockStore)

	res, total, err := mockComp.ListMeteringByUserIDAndDate(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.NotNil(t, res)
}

func TestMeteringComponent_GetMeteringStatByDate(t *testing.T) {
	ctx := context.TODO()

	req := types.ActStatementsReq{}
	data := []map[string]interface{}{}

	mockStore := mockdb.NewMockAccountMeteringStore(t)

	mockStore.EXPECT().GetStatByDate(ctx, req).Return(data, nil)

	mockComp := NewTestMeteringComponent(mockStore)
	res, err := mockComp.GetMeteringStatByDate(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
}

func TestMeteringComponent_GetMeteringByEventUUID(t *testing.T) {
	ctx := context.TODO()

	mockStore := mockdb.NewMockAccountMeteringStore(t)

	uuid := uuid.New()

	mockStore.EXPECT().GetByEventUUID(ctx, uuid).Return(&database.AccountMetering{}, nil)

	mockComp := NewTestMeteringComponent(mockStore)
	res, err := mockComp.GetMeteringByEventUUID(ctx, uuid)
	require.Nil(t, err)
	require.NotNil(t, res)
}

func TestMeteringComponent_GetMeteringByCustomerIDAndDate(t *testing.T) {
	ctx := context.TODO()

	mockStore := mockdb.NewMockAccountMeteringStore(t)

	customerid := "test-customer-id"
	recordedat := time.Now()

	mockStore.EXPECT().FindByCustomerIDAndRecordAtInMin(ctx, customerid, recordedat).Return(&database.AccountMetering{}, nil)

	mockComp := NewTestMeteringComponent(mockStore)
	res, err := mockComp.FindMeteringByCustomerIDAndRecordAtInMin(ctx, customerid, recordedat)
	require.Nil(t, err)
	require.NotNil(t, res)
}

func TestMeteringComponent_ListStatisticsByUserIDAndDate(t *testing.T) {
	ctx := context.TODO()

	req := types.AcctBillsReq{
		TargetUUID: "test-user-uuid",
		Scene:      types.SceneModelInference,
		StartDate:  "2024-01-01",
		EndDate:    "2024-12-31",
		Per:        10,
		Page:       1,
	}

	mockStatStore := mockdb.NewMockAccountStatisticsStore(t)
	mockStatStore.EXPECT().ListByUserIDAndDate(ctx, req).Return(database.AccountStatisticsRes{}, nil)

	comp := &meteringComponentImpl{
		ass: mockStatStore,
	}

	res, err := comp.ListStatisticsByUserIDAndDate(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
}

func TestMeteringComponent_ListStatisticsDetailByUserID(t *testing.T) {
	ctx := context.TODO()

	req := types.AcctBillsDetailReq{
		TargetUUID: "test-user-uuid",
		Scene:      10,
		StartDate:  "2024-01-01",
		EndDate:    "2024-12-31",
		Per:        10,
		Page:       1,
	}

	mockStatStore := mockdb.NewMockAccountStatisticsStore(t)
	mockStatStore.EXPECT().ListStatisticsDetailByUserID(ctx, req).Return(database.AccountStatisticsDetailRes{}, nil)

	comp := &meteringComponentImpl{
		ass: mockStatStore,
	}

	res, err := comp.ListStatisticsDetailByUserID(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
}

func TestMeteringComponent_GetStatisticsSummary(t *testing.T) {
	ctx := context.TODO()

	req := types.AcctBillsReq{
		TargetUUID: "test-user-uuid",
		Scene:      types.SceneModelInference,
		StartDate:  "2024-01-01",
		EndDate:    "2024-12-31",
	}

	mockStatStore := mockdb.NewMockAccountStatisticsStore(t)
	mockStatStore.EXPECT().SummaryByUserIDAndDate(ctx, req).Return(database.AccountStatisticsSummaryRes{}, nil)

	comp := &meteringComponentImpl{
		ass: mockStatStore,
	}

	res, err := comp.GetStatisticsSummary(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
}
