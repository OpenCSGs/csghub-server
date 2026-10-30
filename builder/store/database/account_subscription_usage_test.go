package database_test

import (
	"context"
	"strconv"
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

	// Subscription usage is only tracked for hardware compute scenes (see
	// utils.IsNeedCheckUserSubscription); starship/serverless statements no
	// longer create usage records. Hardware scenes also run bill resource
	// extraction, which requires a numeric ResourceID backed by an existing
	// SpaceResource.
	sr := &database.SpaceResource{
		Name:      "test-resource",
		Resources: `{"gpu": {"num": "1", "type": "A10"}}`,
		ClusterID: "test-cluster",
	}
	_, err = db.Core.NewInsert().Model(sr).Exec(ctx, sr)
	require.Nil(t, err)
	resourceID := strconv.FormatInt(sr.ID, 10)

	eventUUID := uuid.New()
	statement := database.AccountStatement{
		EventUUID:  eventUUID,
		UserUUID:   "user1",
		Value:      0,
		Scene:      types.SceneSpace,
		ValueType:  types.TokenNumberType,
		Quota:      1000,
		SubBillID:  1,
		ResourceID: resourceID,
	}

	err = stStore.Create(ctx, statement, types.AcctStatementExtra{})
	require.Nil(t, err)

	res, err := usageStore.GetByBillID(ctx, statement.SubBillID, statement.UserUUID, types.SKUCSGHub)
	require.Nil(t, err)
	require.Equal(t, 1, len(res))

	eventUUID = uuid.New()
	statement = database.AccountStatement{
		EventUUID:    eventUUID,
		UserUUID:     "user1",
		Scene:        types.SceneSpace,
		Value:        0,
		ValueType:    types.TokenNumberType,
		Quota:        1000,
		SubBillID:    -1,
		ResourceID:   resourceID,
		ResourceName: "res1",
		CustomerID:   "cus1",
	}
	err = stStore.Create(ctx, statement, types.AcctStatementExtra{})
	require.Nil(t, err)

	res, err = usageStore.GetByBillMonth(ctx, time.Now().Format("2006-01"), statement.UserUUID, types.SKUCSGHub)
	require.Nil(t, err)
	require.Equal(t, 1, len(res))
	require.Equal(t, statement.ResourceID, res[0].ResourceID)
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
