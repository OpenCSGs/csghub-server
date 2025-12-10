package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountSubscriptionUsageStore_CURD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	stStore := database.NewAccountStatementStoreWithDB(db)
	usageStore := database.NewAccountSubscriptionUsageWithDB(db)
	userStore := database.NewAccountUserStoreWithDB(db)

	acctUser := database.AccountUser{
		UserUUID:    "user1",
		Balance:     float64(200),
		CashBalance: float64(200),
	}
	err := userStore.Create(ctx, acctUser)
	require.Nil(t, err)

	eventUUID := uuid.New()
	statement := database.AccountStatement{
		EventUUID: eventUUID,
		UserUUID:  "user1",
		Value:     0,
		Scene:     types.SceneStarship,
		ValueType: types.TokenNumberType,
		Quota:     1000,
		SubBillID: 1,
	}

	err = stStore.Create(ctx, statement)
	require.Nil(t, err)

	res, err := usageStore.GetByBillID(ctx, statement.SubBillID, statement.UserUUID, types.SKUStarship)
	require.Nil(t, err)
	require.Equal(t, len(res), 1)

	eventUUID = uuid.New()
	statement = database.AccountStatement{
		EventUUID:    eventUUID,
		UserUUID:     "user1",
		Scene:        types.SceneStarship,
		Value:        0,
		ValueType:    types.TokenNumberType,
		Quota:        1000,
		SubBillID:    -1,
		ResourceID:   "res1",
		ResourceName: "res1",
		CustomerID:   "cus1",
	}
	err = stStore.Create(ctx, statement)
	require.Nil(t, err)

	res, err = usageStore.GetByBillMonth(ctx, time.Now().Format("2006-01"), statement.UserUUID, types.SKUStarship)
	require.Nil(t, err)
	require.Equal(t, len(res), 1)
	require.Equal(t, res[0].ResourceID, statement.ResourceID)
}

func TestAccountSubscriptionUsageStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	usageStore := database.NewAccountSubscriptionUsageWithDB(db)

	input := &database.AccountSubscriptionUsage{
		UserUUID:     "test-user",
		ResourceID:   "rid",
		ResourceName: "rname",
		CustomerID:   "cid",
		Used:         1,
		Quota:        10,
	}

	res, err := usageStore.Create(ctx, input)
	require.Nil(t, err)
	require.NotNil(t, res)
}
