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

func TestAccountPresentStore_AddPresent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPresentStoreWithDB(db)
	_, err := db.Core.NewInsert().Model(&database.AccountUser{
		UserUUID: "foo",
		Balance:  5,
	}).Exec(ctx)
	require.Nil(t, err)

	err = store.AddPresent(ctx, database.AccountPresent{
		UserUUID: "foo",
		Value:    123,
	}, database.AccountStatement{
		UserUUID: "foo",
		Value:    123,
	})
	require.Nil(t, err)

	present := &database.AccountPresent{}
	stat := &database.AccountStatement{}
	auser := &database.AccountUser{}

	err = db.Core.NewSelect().Model(present).Where("user_uuid=?", "foo").Scan(ctx)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(stat).Where("user_uuid=?", "foo").Scan(ctx)
	require.Nil(t, err)
	err = db.Core.NewSelect().Model(auser).Where("user_uuid=?", "foo").Scan(ctx)
	require.Nil(t, err)

	require.Equal(t, float64(123), present.Value)
	require.Equal(t, float64(123), stat.Value)
	require.Equal(t, float64(128), auser.Balance)
}

func TestAccountPresentStore_FindPresentByUserAndActivity(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	ps := []database.AccountPresent{
		{UserUUID: "foo", ActivityID: 1, Value: 1},
		{UserUUID: "foo", ActivityID: 2, Value: 2},
		{UserUUID: "bar", ActivityID: 1, Value: 3},
		{UserUUID: "foo", ActivityID: 3, Value: 4},
	}

	for _, p := range ps {
		_, err := db.Core.NewInsert().Model(&p).Exec(ctx)
		require.Nil(t, err)
	}

	store := database.NewAccountPresentStoreWithDB(db)
	p, err := store.FindPresentByUserAndActivity(ctx, "foo", 1, "")
	require.Nil(t, err)
	require.Equal(t, float64(1), p.Value)

}

func TestAccountPresentStore_ListExpiredPresentsByActivityID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPresentStoreWithDB(db)
	activityID := int64(1)

	// Create test presents with different statuses and expiration times
	now := time.Now()
	expiredTime := now.Add(-1 * time.Hour) // 1 hour ago
	futureTime := now.Add(1 * time.Hour)   // 1 hour from now

	presents := []database.AccountPresent{
		{
			EventUUID:       uuid.New(),
			UserUUID:        "user1",
			ActivityID:      activityID,
			Value:           100,
			OpUID:           "op1",
			OpDesc:          "test",
			ParticipantUUID: "participant1",
			ExpireAt:        expiredTime,
			Status:          types.AccountPresentStatusInit,
		},
		{
			EventUUID:       uuid.New(),
			UserUUID:        "user2",
			ActivityID:      activityID,
			Value:           200,
			OpUID:           "op2",
			OpDesc:          "test",
			ParticipantUUID: "participant2",
			ExpireAt:        expiredTime,
			Status:          types.AccountPresentStatusInit,
		},
		{
			EventUUID:       uuid.New(),
			UserUUID:        "user3",
			ActivityID:      activityID,
			Value:           300,
			OpUID:           "op3",
			OpDesc:          "test",
			ParticipantUUID: "participant3",
			ExpireAt:        futureTime, // Not expired
			Status:          types.AccountPresentStatusUsed,
		},
		{
			EventUUID:       uuid.New(),
			UserUUID:        "user4",
			ActivityID:      activityID,
			Value:           400,
			OpUID:           "op4",
			OpDesc:          "test",
			ParticipantUUID: "participant4",
			ExpireAt:        expiredTime,
			Status:          types.AccountPresentStatusInit, // Not used
		},
		{
			EventUUID:       uuid.New(),
			UserUUID:        "user5",
			ActivityID:      activityID,
			Value:           500,
			OpUID:           "op5",
			OpDesc:          "test",
			ParticipantUUID: "participant5",
			ExpireAt:        time.Time{}, // No expiration
			Status:          types.AccountPresentStatusUsed,
		},
	}

	// Insert test data
	for _, present := range presents {
		_, err := db.Core.NewInsert().Model(&present).Exec(ctx)
		require.Nil(t, err)
	}

	// Test: List expired presents by activity ID
	expiredPresents, err := store.ListExpiredPresentsByActivityID(ctx, activityID)
	require.Nil(t, err)
	require.Len(t, expiredPresents, 3) // Only 3 presents should be returned

	// Verify the returned presents are the expired and init ones
	userUUIDs := make(map[string]bool)
	for _, present := range expiredPresents {
		userUUIDs[present.UserUUID] = true
		require.Equal(t, activityID, present.ActivityID)
		require.Equal(t, types.AccountPresentStatusInit, present.Status)
		require.False(t, present.ExpireAt.IsZero())
		require.True(t, present.ExpireAt.Before(now))
	}
	require.True(t, userUUIDs["user1"])
	require.True(t, userUUIDs["user2"])
	require.True(t, userUUIDs["user4"])

	// Test: List expired presents for non-existent activity
	emptyPresents, err := store.ListExpiredPresentsByActivityID(ctx, 999)
	require.Nil(t, err)
	require.Len(t, emptyPresents, 0)
}

func TestAccountPresentStore_HasConsumptionActivity(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPresentStoreWithDB(db)
	userUUID := "test-user"
	startAt := time.Now().Add(-24 * time.Hour) // 24 hours ago

	// Create test account user
	_, err := db.Core.NewInsert().Model(&database.AccountUser{
		UserUUID: userUUID,
		Balance:  1000,
	}).Exec(ctx)
	require.Nil(t, err)

	// Test: No consumption activity initially
	hasActivity, err := store.HasConsumptionActivity(ctx, userUUID, startAt)
	require.Nil(t, err)
	require.False(t, hasActivity)

	// Create consumption statements
	statements := []database.AccountStatement{
		{
			EventUUID: uuid.New(),
			UserUUID:  userUUID,
			Value:     100,
			Scene:     types.ScenePayOrder,
			OpUID:     "op1",
		},
		{
			EventUUID: uuid.New(),
			UserUUID:  userUUID,
			Value:     200,
			Scene:     types.SceneModelInference,
			OpUID:     "op2",
		},
		{
			EventUUID: uuid.New(),
			UserUUID:  userUUID,
			Value:     50,
			Scene:     types.ScenePortalCharge, // Non-consumption scene
			OpUID:     "op3",
		},
	}

	// Insert statements
	for _, statement := range statements {
		_, err := db.Core.NewInsert().Model(&statement).Exec(ctx)
		require.Nil(t, err)
	}

	// Test: Has consumption activity after inserting consumption statements
	hasActivity, err = store.HasConsumptionActivity(ctx, userUUID, startAt)
	require.Nil(t, err)
	require.True(t, hasActivity)

	// Test: No consumption activity when looking at a time range before the statements
	oldStartAt := time.Now().Add(-48 * time.Hour) // 48 hours ago
	hasActivity, err = store.HasConsumptionActivity(ctx, userUUID, oldStartAt)
	require.Nil(t, err)
	require.True(t, hasActivity) // Still true because statements are within this range

	// Test: No consumption activity for different user
	hasActivity, err = store.HasConsumptionActivity(ctx, "different-user", startAt)
	require.Nil(t, err)
	require.False(t, hasActivity)
}

func TestAccountPresentStore_DeductPresent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPresentStoreWithDB(db)
	userUUID := "test-user"
	eventUUID := uuid.New()

	// Create test account user
	_, err := db.Core.NewInsert().Model(&database.AccountUser{
		UserUUID: userUUID,
		Balance:  1000,
	}).Exec(ctx)
	require.Nil(t, err)

	// Create test present
	present := database.AccountPresent{
		EventUUID:       eventUUID,
		UserUUID:        userUUID,
		ActivityID:      1,
		Value:           100,
		OpUID:           "op1",
		OpDesc:          "test present",
		ParticipantUUID: "participant1",
		Status:          types.AccountPresentStatusUsed,
	}
	_, err = db.Core.NewInsert().Model(&present).Exec(ctx)
	require.Nil(t, err)

	// Create corresponding statement
	statement := database.AccountStatement{
		EventUUID: eventUUID,
		UserUUID:  userUUID,
		Value:     100,
		Scene:     types.ScenePortalCharge,
		OpUID:     "op1",
		IsCancel:  false,
	}
	_, err = db.Core.NewInsert().Model(&statement).Exec(ctx)
	require.Nil(t, err)

	// Test: Deduct present successfully
	err = store.CancelPresent(ctx, eventUUID)
	require.Nil(t, err)

	// Verify present status is updated to canceled
	var updatedPresent database.AccountPresent
	err = db.Core.NewSelect().Model(&updatedPresent).Where("event_uuid = ?", eventUUID).Scan(ctx, &updatedPresent)
	require.Nil(t, err)
	require.Equal(t, types.AccountPresentStatusCanceled, updatedPresent.Status)

	// Verify statement is marked as canceled
	var updatedStatement database.AccountStatement
	err = db.Core.NewSelect().Model(&updatedStatement).Where("event_uuid = ?", eventUUID).Scan(ctx, &updatedStatement)
	require.Nil(t, err)
	require.True(t, updatedStatement.IsCancel)

	// Verify user balance is reduced
	var updatedUser database.AccountUser
	err = db.Core.NewSelect().Model(&updatedUser).Where("user_uuid = ?", userUUID).Scan(ctx, &updatedUser)
	require.Nil(t, err)
	require.Equal(t, float64(900), updatedUser.Balance) // 1000 - 100

	// Test: Deduct non-existent present
	nonExistentUUID := uuid.New()
	err = store.CancelPresent(ctx, nonExistentUUID)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "get account present")
}

func TestAccountPresentStore_MarkPresentAsUsed(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPresentStoreWithDB(db)
	userUUID := "test-user"
	eventUUID := uuid.New()

	// Create test account user
	_, err := db.Core.NewInsert().Model(&database.AccountUser{
		UserUUID: userUUID,
		Balance:  1000,
	}).Exec(ctx)
	require.Nil(t, err)

	// Create test present with initial status
	present := database.AccountPresent{
		EventUUID:       eventUUID,
		UserUUID:        userUUID,
		ActivityID:      1,
		Value:           100,
		OpUID:           "op1",
		OpDesc:          "test present",
		ParticipantUUID: "participant1",
		Status:          types.AccountPresentStatusInit,
	}
	_, err = db.Core.NewInsert().Model(&present).Exec(ctx)
	require.Nil(t, err)

	// Test: Mark present as used successfully
	err = store.MarkPresentAsUsed(ctx, eventUUID)
	require.Nil(t, err)

	// Verify present status is updated to used
	var updatedPresent database.AccountPresent
	err = db.Core.NewSelect().Model(&updatedPresent).Where("event_uuid = ?", eventUUID).Scan(ctx, &updatedPresent)
	require.Nil(t, err)
	require.Equal(t, types.AccountPresentStatusUsed, updatedPresent.Status)
	require.Equal(t, eventUUID, updatedPresent.EventUUID)
	require.Equal(t, userUUID, updatedPresent.UserUUID)
	require.Equal(t, float64(100), updatedPresent.Value)

	// Test: Mark non-existent present as used
	nonExistentUUID := uuid.New()
	err = store.MarkPresentAsUsed(ctx, nonExistentUUID)
	require.Nil(t, err) // The method doesn't check if the record exists, it just updates

	// Test: Mark present as used when it's already used (should still work)
	err = store.MarkPresentAsUsed(ctx, eventUUID)
	require.Nil(t, err)

	// Verify status remains used
	err = db.Core.NewSelect().Model(&updatedPresent).Where("event_uuid = ?", eventUUID).Scan(ctx, &updatedPresent)
	require.Nil(t, err)
	require.Equal(t, types.AccountPresentStatusUsed, updatedPresent.Status)
}

func TestAccountPresentStore_MarkPresentAsUsed_MultiplePresents(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPresentStoreWithDB(db)
	userUUID := "test-user"
	eventUUID1 := uuid.New()
	eventUUID2 := uuid.New()

	// Create test account user
	_, err := db.Core.NewInsert().Model(&database.AccountUser{
		UserUUID: userUUID,
		Balance:  1000,
	}).Exec(ctx)
	require.Nil(t, err)

	// Create multiple test presents
	presents := []database.AccountPresent{
		{
			EventUUID:       eventUUID1,
			UserUUID:        userUUID,
			ActivityID:      1,
			Value:           100,
			OpUID:           "op1",
			OpDesc:          "test present 1",
			ParticipantUUID: "participant1",
			Status:          types.AccountPresentStatusInit,
		},
		{
			EventUUID:       eventUUID2,
			UserUUID:        userUUID,
			ActivityID:      2,
			Value:           200,
			OpUID:           "op2",
			OpDesc:          "test present 2",
			ParticipantUUID: "participant2",
			Status:          types.AccountPresentStatusInit,
		},
	}

	// Insert presents
	for _, present := range presents {
		_, err = db.Core.NewInsert().Model(&present).Exec(ctx)
		require.Nil(t, err)
	}

	// Test: Mark only the first present as used
	err = store.MarkPresentAsUsed(ctx, eventUUID1)
	require.Nil(t, err)

	// Verify only the first present is updated
	var updatedPresent1 database.AccountPresent
	err = db.Core.NewSelect().Model(&updatedPresent1).Where("event_uuid = ?", eventUUID1).Scan(ctx, &updatedPresent1)
	require.Nil(t, err)
	require.Equal(t, types.AccountPresentStatusUsed, updatedPresent1.Status)

	var updatedPresent2 database.AccountPresent
	err = db.Core.NewSelect().Model(&updatedPresent2).Where("event_uuid = ?", eventUUID2).Scan(ctx, &updatedPresent2)
	require.Nil(t, err)
	require.Equal(t, types.AccountPresentStatusInit, updatedPresent2.Status) // Should remain unchanged

	// Test: Mark the second present as used
	err = store.MarkPresentAsUsed(ctx, eventUUID2)
	require.Nil(t, err)

	// Verify both presents are now used
	err = db.Core.NewSelect().Model(&updatedPresent1).Where("event_uuid = ?", eventUUID1).Scan(ctx, &updatedPresent1)
	require.Nil(t, err)
	require.Equal(t, types.AccountPresentStatusUsed, updatedPresent1.Status)

	err = db.Core.NewSelect().Model(&updatedPresent2).Where("event_uuid = ?", eventUUID2).Scan(ctx, &updatedPresent2)
	require.Nil(t, err)
	require.Equal(t, types.AccountPresentStatusUsed, updatedPresent2.Status)
}
