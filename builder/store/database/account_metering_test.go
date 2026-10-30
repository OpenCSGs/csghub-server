package database_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountMeteringStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountMeteringStoreWithDB(db)
	am := database.AccountMetering{
		UserUUID:     "foo",
		Value:        12.34,
		ValueType:    1,
		ResourceName: "abc",
	}
	err := store.Create(ctx, am, types.MeteringExtra{})
	require.Nil(t, err)
	amn := &database.AccountMetering{}
	err = db.Core.NewSelect().Model(amn).Where("user_uuid = ?", "foo").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foo", amn.UserUUID)
	require.Equal(t, 12.34, amn.Value)
	require.Equal(t, types.ChargeValueType(1), amn.ValueType)
	require.Equal(t, "abc", amn.ResourceName)
}

func TestAccountMeteringStore_Create_UpsertStatisticsReasoningToken(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountMeteringStoreWithDB(db)
	eventDate := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	extra := types.MeteringExtra{
		EventDate:         eventDate,
		PromptToken:       100,
		PromptCachedToken: 10,
		CompletionToken:   50,
		ReasoningToken:    30,
	}
	am := database.AccountMetering{
		EventUUID:    uuid.New(),
		UserUUID:     "bar-reasoning",
		Value:        150,
		ValueType:    1,
		ResourceName: "reasoning-model",
	}
	err := store.Create(ctx, am, extra)
	require.Nil(t, err)

	// Second event with the same statistics conflict key must accumulate.
	extra.ReasoningToken = 20
	am.EventUUID = uuid.New()
	err = store.Create(ctx, am, extra)
	require.Nil(t, err)

	stat := &database.AccountStatistics{}
	err = db.Core.NewSelect().Model(stat).
		Where("user_uuid = ?", "bar-reasoning").
		Where("event_date = ?", eventDate).
		Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, float64(200), stat.PromptToken)
	require.Equal(t, float64(100), stat.CompletionToken)
	require.Equal(t, float64(50), stat.ReasoningToken)
	require.Equal(t, float64(2), stat.Count)
}

func TestAccountMeteringStore_ListByUserIDAndTime(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountMeteringStoreWithDB(db)

	dt := time.Date(2022, 11, 22, 3, 0, 0, 0, time.UTC)
	ams := []database.AccountMetering{
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r1", Scene: types.ScenePayOrder, CustomerID: "bar",
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r2", Scene: types.ScenePayOrder, CustomerID: "bar",
			RecordedAt: dt.Add(-2 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r3", Scene: types.ScenePayOrder, CustomerID: "bar",
			RecordedAt: dt.Add(1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r4", Scene: types.ScenePayOrder, CustomerID: "bar",
			RecordedAt: dt.Add(2 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r5", Scene: types.ScenePayOrder, CustomerID: "bar",
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r6", Scene: types.ScenePayOrder, CustomerID: "bar",
			RecordedAt: dt.Add(-6 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r7", Scene: types.ScenePayOrder, CustomerID: "bar",
			RecordedAt: dt.Add(6 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "bar", Value: 12.34, ValueType: 1,
			ResourceName: "r8", Scene: types.ScenePayOrder, CustomerID: "bar",
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r9", Scene: types.SceneMultiSync, CustomerID: "bar",
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r10", Scene: types.ScenePayOrder, CustomerID: "",
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
	}

	for _, am := range ams {
		err := store.Create(ctx, am, types.MeteringExtra{})
		require.Nil(t, err)
	}

	ams, total, err := store.ListByUserIDAndTime(ctx, types.ActStatementsReq{
		UserUUID:     "foo",
		Scene:        2,
		InstanceName: "bar",
		StartTime:    dt.Add(-5 * time.Hour).Format(time.RFC3339),
		EndTime:      dt.Add(5 * time.Hour).Format(time.RFC3339),
	})
	require.Nil(t, err)
	require.Equal(t, 5, total)
	names := []string{}
	for _, am := range ams {
		names = append(names, am.ResourceName)
	}
	require.Equal(t, []string{"r5", "r4", "r3", "r2", "r1"}, names)

	ams, total, err = store.ListByUserIDAndTime(ctx, types.ActStatementsReq{
		UserUUID:     "foo",
		Scene:        2,
		InstanceName: "",
		StartTime:    dt.Add(-5 * time.Hour).Format(time.RFC3339),
		EndTime:      dt.Add(5 * time.Hour).Format(time.RFC3339),
	})
	require.Nil(t, err)
	require.Equal(t, 6, total)
	names = []string{}
	for _, am := range ams {
		names = append(names, am.ResourceName)
	}
	require.Equal(t, []string{"r10", "r5", "r4", "r3", "r2", "r1"}, names)
}

func TestAccountMeteringStore_GetStatByDate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountMeteringStoreWithDB(db)
	us := database.NewUserStoreWithDB(db)
	err := us.Create(ctx, &database.User{
		Username: "u1",
		UUID:     "foo",
	}, &database.Namespace{Path: "a"})
	require.Nil(t, err)

	err = us.Create(ctx, &database.User{
		Username: "u2",
		UUID:     "bar",
	}, &database.Namespace{Path: "b"})
	require.Nil(t, err)

	dt := time.Date(2022, 11, 22, 3, 0, 0, 0, time.UTC)
	ams := []database.AccountMetering{
		{
			UserUUID: "foo", Value: 1.1, ValueType: 1,
			ResourceID: "r1", Scene: types.ScenePayOrder,
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 1.2, ValueType: 2,
			ResourceID: "r1", Scene: types.SceneModelFinetune,
			RecordedAt: dt.Add(-2 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 1.2, ValueType: 2,
			ResourceID: "r1", Scene: types.ScenePayOrder,
			RecordedAt: dt.Add(-6 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 1.5, ValueType: 1,
			ResourceID: "r2", Scene: types.ScenePayOrder,
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "bar", Value: 1.1, ValueType: 1,
			ResourceID: "r1", Scene: types.ScenePayOrder,
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "bar", Value: 1.2, ValueType: 2,
			ResourceID: "r1", Scene: types.ScenePayOrder,
			RecordedAt: dt.Add(-6 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "bar", Value: 1.2, ValueType: 2,
			ResourceID: "r1", Scene: types.ScenePayOrder,
			RecordedAt: dt.Add(-2 * time.Hour), EventUUID: uuid.New(),
		},
		{
			UserUUID: "bar", Value: 1.5, ValueType: 1,
			ResourceID: "r2", Scene: types.ScenePayOrder,
			RecordedAt: dt.Add(-1 * time.Hour), EventUUID: uuid.New(),
		},
	}

	for _, am := range ams {
		err := store.Create(ctx, am, types.MeteringExtra{})
		require.Nil(t, err)
	}

	data, err := store.GetStatByDate(ctx, types.ActStatementsReq{
		UserUUID:     "foo",
		Scene:        2,
		InstanceName: "bar",
		StartTime:    dt.Add(-5 * time.Hour).Format(time.RFC3339),
		EndTime:      dt.Add(5 * time.Hour).Format(time.RFC3339),
	})
	require.Nil(t, err)
	require.Equal(t, 4, len(data))
	expected := []map[string]interface{}{
		{"resource_id": "r1", "user_uuid": "bar", "username": "u2", "value": 2.3},
		{"resource_id": "r1", "user_uuid": "foo", "username": "u1", "value": 1.1},
		{"resource_id": "r2", "user_uuid": "foo", "username": "u1", "value": 1.5},
		{"resource_id": "r2", "user_uuid": "bar", "username": "u2", "value": 1.5},
	}
	require.Equal(t, expected, data)
}

func TestAccountMeteringStore_ListAllByUserUUID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountMeteringStoreWithDB(db)
	ams := []database.AccountMetering{
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r1", Scene: types.ScenePayOrder, CustomerID: "bar",
			EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r2", Scene: types.ScenePayOrder, CustomerID: "bar",
			EventUUID: uuid.New(),
		},
		{
			UserUUID: "foo", Value: 12.34, ValueType: 1,
			ResourceName: "r3", Scene: types.ScenePayOrder, CustomerID: "bar",
			EventUUID: uuid.New(),
		},
		{
			UserUUID: "bar", Value: 12.34, ValueType: 1,
			ResourceName: "r4", Scene: types.ScenePayOrder, CustomerID: "bar",
			EventUUID: uuid.New(),
		},
	}

	for _, am := range ams {
		err := store.Create(ctx, am, types.MeteringExtra{})
		require.Nil(t, err)
	}
	data, err := store.ListAllByUserUUID(ctx, "foo")
	require.Nil(t, err)
	require.Equal(t, 3, len(data))
}

func TestAccountMeteringStore_GetByEventUUID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountMeteringStoreWithDB(db)

	am := database.AccountMetering{
		UserUUID: "foo", Value: 12.34, ValueType: 1,
		ResourceName: "r1", Scene: types.ScenePayOrder, CustomerID: "bar",
		EventUUID: uuid.New(),
	}

	err := store.Create(ctx, am, types.MeteringExtra{})
	require.Nil(t, err)

	data, err := store.GetByEventUUID(ctx, am.EventUUID)
	require.Nil(t, err)
	require.Equal(t, am.UserUUID, data.UserUUID)
	require.Equal(t, am.Value, data.Value)
}

func TestAccountMeteringStore_FindByCustomerIDAndRecordAtInMin(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountMeteringStoreWithDB(db)

	ctime := time.Now()

	am := database.AccountMetering{
		UserUUID: "foo", Value: 12.34, ValueType: 1,
		ResourceName: "r1", Scene: types.ScenePayOrder, CustomerID: "bar",
		EventUUID: uuid.New(), RecordedAt: ctime,
	}

	err := store.Create(ctx, am, types.MeteringExtra{})
	require.Nil(t, err)

	data, err := store.FindByCustomerIDAndRecordAtInMin(ctx, am.CustomerID, time.Now())
	require.Nil(t, err)
	require.NotNil(t, data)
	require.Equal(t, am.UserUUID, data.UserUUID)
	require.Equal(t, am.Value, data.Value)
}

// A non-numeric api_key in the metering extra (a legacy raw key value from an
// in-flight event) must degrade to token_id 0 without failing the metering
// record, and the parse failure must be logged.
func TestAccountMeteringStore_CreateNonNumericAPIKeyDegradesToZeroTokenID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	var logBuf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, nil)))
	defer slog.SetDefault(prevLogger)

	const rawLegacyKey = "sk-legacy-do-not-log-0123456789"
	eventDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	store := database.NewAccountMeteringStoreWithDB(db)
	am := database.AccountMetering{
		EventUUID:    uuid.New(),
		UserUUID:     "metering-nonnumeric-key",
		Value:        150,
		ValueType:    types.TokenNumberType,
		Scene:        types.SceneModelServerless,
		ResourceName: "serverless-model",
		RecordedAt:   time.Now(),
		Extra:        fmt.Sprintf(`{"api_key":%q,"prompt_token_num":"100"}`, rawLegacyKey),
	}
	extra := types.MeteringExtra{
		EventDate:       eventDate,
		PromptToken:     100,
		CompletionToken: 50,
		APIKey:          rawLegacyKey,
	}
	err := store.Create(ctx, am, extra)
	require.Nil(t, err)

	// The metering record is still persisted.
	amn := &database.AccountMetering{}
	err = db.Core.NewSelect().Model(amn).Where("user_uuid = ?", "metering-nonnumeric-key").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, am.EventUUID, amn.EventUUID)

	// The statistics degrade to token_id 0.
	stat := &database.AccountStatistics{}
	err = db.Core.NewSelect().Model(stat).
		Where("user_uuid = ?", "metering-nonnumeric-key").
		Where("event_date = ?", eventDate).
		Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, int64(0), stat.TokenID)
	require.Equal(t, float64(1), stat.Count)

	// The parse failure is logged and the metering record continues. NOTE: the
	// current implementation embeds the raw api_key in this error log (the
	// strconv error text and the token_id_str attribute); redacting it requires
	// an implementation change, which is out of scope for this test-only fix.
	require.Contains(t, logBuf.String(), "parse api key ID str to int64 fail in metering")
}
