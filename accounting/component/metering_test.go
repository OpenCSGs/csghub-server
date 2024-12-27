package component

import (
	"context"
	"testing"

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
	req := types.METERING_EVENT{
		Uuid:       uid,
		UserUUID:   "test-user-uuid",
		Value:      100,
		ValueType:  10,
		Scene:      1,
		OpUID:      "test-op-uid",
		ResourceID: "test-ID",
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
		SkuUnitType:  utils.GetSkuUnitTypeByScene(types.SceneType(req.Scene)),
	}

	mockStore := mockdb.NewMockAccountMeteringStore(t)
	mockStore.EXPECT().Create(ctx, data).Return(nil)

	mockComp := NewTestMeteringComponent(mockStore)

	err := mockComp.SaveMeteringEventRecord(ctx, &req)

	require.Nil(t, err)
}

func TestMeteringComponent_ListMeteringByUserIDAndDate(t *testing.T) {
	ctx := context.TODO()

	req := types.ACCT_STATEMENTS_REQ{
		UserUUID:  "test-user-uuid",
		Scene:     int(types.SceneModelInference),
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

	req := types.ACCT_STATEMENTS_REQ{}
	data := []map[string]interface{}{}

	mockStore := mockdb.NewMockAccountMeteringStore(t)

	mockStore.EXPECT().GetStatByDate(ctx, req).Return(data, nil)

	mockComp := NewTestMeteringComponent(mockStore)
	res, err := mockComp.GetMeteringStatByDate(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
}
