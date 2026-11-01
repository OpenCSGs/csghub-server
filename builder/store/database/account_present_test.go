package database_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
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

func TestAccountPresentStore_ListAutoExpirePresents(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPresentStoreWithDB(db)
	activityID := int64(1005)
	now := time.Now()
	grantedStart := now.Add(-45 * 24 * time.Hour)
	grantedEnd := now.Add(-35 * 24 * time.Hour) // exclusive
	validDays := 30

	seedPresent := func(userUUID string, actID int64, value float64, status types.AccountPresentStatus, expireAt, grantedAt time.Time) int64 {
		present := database.AccountPresent{
			EventUUID:  uuid.New(),
			UserUUID:   userUUID,
			ActivityID: actID,
			Value:      value,
			OpUID:      "op",
			OpDesc:     "test",
			Status:     status,
			ExpireAt:   expireAt,
		}
		present.CreatedAt = grantedAt
		_, err := db.Core.NewInsert().Model(&present).Exec(ctx)
		require.Nil(t, err)
		return present.ID
	}

	grantedLongAgo := now.Add(-40 * 24 * time.Hour)
	hitID := seedPresent("u-hit", activityID, 5000, types.AccountPresentStatusInit, time.Time{}, grantedLongAgo)
	seedPresent("u-hit", 1003, 9900, types.AccountPresentStatusInit, time.Time{}, grantedLongAgo) // other activity
	seedPresent("u-fresh", activityID, 5000, types.AccountPresentStatusInit, time.Time{}, now.Add(-24*time.Hour))
	seedPresent("u-too-old", activityID, 5000, types.AccountPresentStatusInit, time.Time{}, now.Add(-400*24*time.Hour))
	seedPresent("u-processed", activityID, 5000, types.AccountPresentStatusInit, now.Add(-24*time.Hour), grantedLongAgo) // expire_at already backfilled
	seedPresent("u-used", activityID, 5000, types.AccountPresentStatusUsed, time.Time{}, grantedLongAgo)

	presents, err := store.ListAutoExpirePresents(ctx, activityID, grantedStart, grantedEnd, validDays, 0, 0)
	require.Nil(t, err)
	require.Len(t, presents, 1)
	require.Equal(t, hitID, presents[0].ID)
	require.Equal(t, "u-hit", presents[0].UserUUID)
	require.Equal(t, activityID, presents[0].ActivityID)
	require.Equal(t, float64(5000), presents[0].Value)

	require.WithinDuration(t, grantedLongAgo, presents[0].CreatedAt, time.Second)
	require.Equal(t, time.Duration(validDays)*24*time.Hour, presents[0].ExpireAt.Sub(presents[0].CreatedAt))

	// presents of an activity without the per-user unique index are returned
	// row by row, without aggregation.
	firstID := seedPresent("u-multi", 7, 100, types.AccountPresentStatusInit, time.Time{}, grantedLongAgo)
	secondID := seedPresent("u-multi", 7, 200, types.AccountPresentStatusInit, time.Time{}, grantedLongAgo)
	multi, err := store.ListAutoExpirePresents(ctx, 7, grantedStart, grantedEnd, validDays, 0, 0)
	require.Nil(t, err)
	require.Len(t, multi, 2)
	require.Equal(t, firstID, multi[0].ID)
	require.Equal(t, secondID, multi[1].ID)
	require.Equal(t, float64(100), multi[0].Value)
	require.Equal(t, float64(200), multi[1].Value)

	// Keyset pagination advances strictly by ID without OFFSET.
	firstPage, err := store.ListAutoExpirePresents(ctx, 7, grantedStart, grantedEnd, validDays, 0, 1)
	require.Nil(t, err)
	require.Len(t, firstPage, 1)
	require.Equal(t, firstID, firstPage[0].ID)

	secondPage, err := store.ListAutoExpirePresents(ctx, 7, grantedStart, grantedEnd, validDays, firstPage[0].ID, 1)
	require.Nil(t, err)
	require.Len(t, secondPage, 1)
	require.Equal(t, secondID, secondPage[0].ID)

	lastPage, err := store.ListAutoExpirePresents(ctx, 7, grantedStart, grantedEnd, validDays, secondPage[0].ID, 1)
	require.Nil(t, err)
	require.Empty(t, lastPage)
}

func TestAccountPresentStore_DeductExpiredPresentBalance(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPresentStoreWithDB(db)
	const activityID = int64(1005)
	const presentValue = 5000.0
	expireAt := time.Now().Add(-24 * time.Hour)

	seedPresent := func(userUUID string, actID int64, status types.AccountPresentStatus) int64 {
		present := database.AccountPresent{
			EventUUID:  uuid.New(),
			UserUUID:   userUUID,
			ActivityID: actID,
			Value:      presentValue,
			OpUID:      "op",
			OpDesc:     "test",
			Status:     status,
		}
		_, err := db.Core.NewInsert().Model(&present).Exec(ctx)
		require.Nil(t, err)
		return present.ID
	}

	newStatement := func(userUUID string) database.AccountStatement {
		return database.AccountStatement{
			EventUUID:  uuid.New(),
			UserUUID:   userUUID,
			Scene:      types.SceneExpiredNewUserPresent,
			OpUID:      "system",
			EventDate:  time.Now().In(config.GetGlobalTimeZone()),
			RecordedAt: time.Now().UTC(),
		}
	}

	getBalance := func(userUUID string) float64 {
		var accountUser database.AccountUser
		err := db.Core.NewSelect().Model(&accountUser).Where("user_uuid = ?", userUUID).Scan(ctx)
		require.Nil(t, err)
		return accountUser.Balance
	}

	statementCount := func(userUUID string) int {
		count, err := db.Core.NewSelect().Model((*database.AccountStatement)(nil)).
			Where("user_uuid = ? AND scene = ?", userUUID, types.SceneExpiredNewUserPresent).Count(ctx)
		require.Nil(t, err)
		return count
	}

	assertExpireAtBackfilled := func(userUUID string, actID int64, backfilled bool) {
		var present database.AccountPresent
		err := db.Core.NewSelect().Model(&present).
			Where("user_uuid = ? AND activity_id = ?", userUUID, actID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, backfilled, !present.ExpireAt.IsZero())
		if backfilled {
			require.Equal(t, types.AccountPresentStatusCanceled, present.Status)
		}
	}

	t.Run("balance above present value deducts the present value", func(t *testing.T) {
		userUUID := "deduct-partial"
		_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: userUUID, Balance: 8000}).Exec(ctx)
		require.Nil(t, err)
		hitID := seedPresent(userUUID, activityID, types.AccountPresentStatusInit)
		otherID := seedPresent(userUUID, 7, types.AccountPresentStatusInit) // other activity must stay untouched

		deducted, err := store.DeductExpiredPresentBalance(ctx, database.AutoExpirePresent{
			ID:         hitID,
			UserUUID:   userUUID,
			ActivityID: activityID,
			Value:      presentValue,
			ExpireAt:   expireAt,
		}, newStatement(userUUID))
		require.Nil(t, err)
		require.Equal(t, presentValue, deducted)
		require.Equal(t, float64(3000), getBalance(userUUID))
		require.Equal(t, 1, statementCount(userUUID))
		assertExpireAtBackfilled(userUUID, activityID, true)
		assertExpireAtBackfilled(userUUID, 7, false)

		var untouchedPresent database.AccountPresent
		err = db.Core.NewSelect().Model(&untouchedPresent).Where("id = ?", otherID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, types.AccountPresentStatusInit, untouchedPresent.Status)

		var stmt database.AccountStatement
		err = db.Core.NewSelect().Model(&stmt).
			Where("user_uuid = ? AND scene = ?", userUUID, types.SceneExpiredNewUserPresent).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, -presentValue, stmt.Value)
		require.Equal(t, -presentValue, stmt.RegularValue)
		require.Equal(t, float64(8000), stmt.BalanceValuePrev)
		require.Equal(t, float64(3000), stmt.BalanceValue)
		require.Equal(t, types.ChargeBalance, stmt.BalanceType)
	})

	t.Run("balance below present value deducts the balance to zero", func(t *testing.T) {
		userUUID := "deduct-to-zero"
		_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: userUUID, Balance: 3000}).Exec(ctx)
		require.Nil(t, err)
		hitID := seedPresent(userUUID, activityID, types.AccountPresentStatusInit)

		deducted, err := store.DeductExpiredPresentBalance(ctx, database.AutoExpirePresent{
			ID:         hitID,
			UserUUID:   userUUID,
			ActivityID: activityID,
			Value:      presentValue,
			ExpireAt:   expireAt,
		}, newStatement(userUUID))
		require.Nil(t, err)
		require.Equal(t, float64(3000), deducted)
		require.Equal(t, float64(0), getBalance(userUUID))
		require.Equal(t, 1, statementCount(userUUID))
		assertExpireAtBackfilled(userUUID, activityID, true)
	})

	t.Run("zero balance user is skipped but marked processed", func(t *testing.T) {
		userUUID := "deduct-zero-balance"
		_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: userUUID, Balance: 0}).Exec(ctx)
		require.Nil(t, err)
		hitID := seedPresent(userUUID, activityID, types.AccountPresentStatusInit)

		deducted, err := store.DeductExpiredPresentBalance(ctx, database.AutoExpirePresent{
			ID:         hitID,
			UserUUID:   userUUID,
			ActivityID: activityID,
			Value:      presentValue,
			ExpireAt:   expireAt,
		}, newStatement(userUUID))
		require.Nil(t, err)
		require.Equal(t, float64(0), deducted)
		require.Equal(t, float64(0), getBalance(userUUID))
		require.Equal(t, 0, statementCount(userUUID))
		assertExpireAtBackfilled(userUUID, activityID, true)
	})

	t.Run("negative balance user is not deducted", func(t *testing.T) {
		userUUID := "deduct-negative-balance"
		_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: userUUID, Balance: -100}).Exec(ctx)
		require.Nil(t, err)
		hitID := seedPresent(userUUID, activityID, types.AccountPresentStatusInit)

		deducted, err := store.DeductExpiredPresentBalance(ctx, database.AutoExpirePresent{
			ID:         hitID,
			UserUUID:   userUUID,
			ActivityID: activityID,
			Value:      presentValue,
			ExpireAt:   expireAt,
		}, newStatement(userUUID))
		require.Nil(t, err)
		require.Equal(t, float64(0), deducted)
		require.Equal(t, float64(-100), getBalance(userUUID))
		require.Equal(t, 0, statementCount(userUUID))
		assertExpireAtBackfilled(userUUID, activityID, true)
	})

	t.Run("missing account user only marks presents processed", func(t *testing.T) {
		userUUID := "deduct-missing-account"
		hitID := seedPresent(userUUID, activityID, types.AccountPresentStatusInit)

		deducted, err := store.DeductExpiredPresentBalance(ctx, database.AutoExpirePresent{
			ID:         hitID,
			UserUUID:   userUUID,
			ActivityID: activityID,
			Value:      presentValue,
			ExpireAt:   expireAt,
		}, newStatement(userUUID))
		require.Nil(t, err)
		require.Equal(t, float64(0), deducted)
		require.Equal(t, 0, statementCount(userUUID))
		assertExpireAtBackfilled(userUUID, activityID, true)
	})

	t.Run("already processed present is skipped without deduction", func(t *testing.T) {
		userUUID := "deduct-already-processed"
		_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: userUUID, Balance: 8000}).Exec(ctx)
		require.Nil(t, err)
		hitID := seedPresent(userUUID, activityID, types.AccountPresentStatusInit)

		firstDeducted, err := store.DeductExpiredPresentBalance(ctx, database.AutoExpirePresent{
			ID:         hitID,
			UserUUID:   userUUID,
			ActivityID: activityID,
			Value:      presentValue,
			ExpireAt:   expireAt,
		}, newStatement(userUUID))
		require.Nil(t, err)
		require.Equal(t, presentValue, firstDeducted)
		require.Equal(t, float64(3000), getBalance(userUUID))
		require.Equal(t, 1, statementCount(userUUID))

		secondDeducted, err := store.DeductExpiredPresentBalance(ctx, database.AutoExpirePresent{
			ID:         hitID,
			UserUUID:   userUUID,
			ActivityID: activityID,
			Value:      presentValue,
			ExpireAt:   expireAt,
		}, newStatement(userUUID))
		require.ErrorIs(t, err, database.ErrAutoExpirePresentAlreadyProcessed)
		require.Zero(t, secondDeducted)
		require.Equal(t, float64(3000), getBalance(userUUID))
		require.Equal(t, 1, statementCount(userUUID))
		assertExpireAtBackfilled(userUUID, activityID, true)
	})
}

func newNewUserPresentGift(userUUID string, activityID int64, voucherValue float64, beginDate, endDate time.Time) database.NewUserPresentGift {
	eventUUID := uuid.New()
	return database.NewUserPresentGift{
		Present: database.AccountPresent{
			EventUUID:  eventUUID,
			UserUUID:   userUUID,
			ActivityID: activityID,
			Value:      voucherValue * 2,
			// the component records gift presents as already canceled so the
			// legacy auto-expire scan never deducts their value from balance
			Status:   types.AccountPresentStatusCanceled,
			ExpireAt: time.Now(),
			OpUID:    "op-user",
			OpDesc:   "new user registration gift",
		},
		Vouchers: []database.AccountVoucher{
			{
				TargetType:  database.UserNamespace,
				TargetUUID:  userUUID,
				TargetName:  "new-user",
				Total:       voucherValue,
				BeginDate:   beginDate,
				EndDate:     endDate,
				Status:      types.VoucherStatusActive,
				Notes:       "new user registration gift",
				IssueUUID:   "system",
				IssueName:   "system",
				VoucherType: types.VoucherTypeCompute,
				VoucherName: "compute gift",
				BatchNo:     eventUUID.String(),
			},
			{
				TargetType:  database.UserNamespace,
				TargetUUID:  userUUID,
				TargetName:  "new-user",
				Total:       voucherValue,
				BeginDate:   beginDate,
				EndDate:     endDate,
				Status:      types.VoucherStatusActive,
				Notes:       "new user registration gift",
				IssueUUID:   "system",
				IssueName:   "system",
				VoucherType: types.VoucherTypeToken,
				VoucherName: "token gift",
				BatchNo:     eventUUID.String(),
			},
		},
		PlanResourceID: database.CSGLiteMonthlyResourceID,
		License: &types.SubscriptionLicenseTemplate{
			Company: "new-user",
			Email:   "new-user@example.com",
			Product: types.LicenseProductCSGLite,
			Edition: "Enterprise",
			MaxUser: 1,
		},
	}
}

// seedNewUserPresentActivity inserts an active activity whose grant window
// contains now, so the new user present tests can request it. The voucher value
// is the server-side amount the store applies to each gifted voucher.
func seedNewUserPresentActivity(t *testing.T, ctx context.Context, db *database.DB, activityID int64) {
	t.Helper()
	now := time.Now()
	_, err := db.Core.NewInsert().Model(&database.AccountActivity{
		ActID:        activityID,
		ActCode:      fmt.Sprintf("test_activity_%d", activityID),
		ActName:      fmt.Sprintf("Test Activity %d", activityID),
		Status:       types.ActivityStatusActive,
		BeginAt:      now.Add(-time.Hour),
		EndAt:        now.Add(24 * time.Hour),
		VoucherValue: 100,
	}).Exec(ctx)
	require.NoError(t, err)
}

func TestAccountPresentStore_SendNewUserPresent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewAccountPresentStoreWithDB(db)

	userUUID := "new-user-gift"
	activityID := int64(9101)
	seedNewUserPresentActivity(t, ctx, db, activityID)

	_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: userUUID, Balance: 77}).Exec(ctx)
	require.Nil(t, err)

	beginDate := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	endDate := beginDate.AddDate(0, 1, 0)

	// the caller-provided amount must be ignored: the activity's voucher_value
	// is the only source of truth for the present and voucher amounts
	awarded, err := store.SendNewUserPresent(ctx, newNewUserPresentGift(userUUID, activityID, 999, beginDate, endDate))
	require.Nil(t, err)
	require.True(t, awarded)

	var present database.AccountPresent
	require.Nil(t, db.Core.NewSelect().Model(&present).Where("user_uuid = ? and activity_id = ?", userUUID, activityID).Scan(ctx))
	require.Equal(t, float64(200), present.Value)
	require.Equal(t, types.AccountPresentStatusCanceled, present.Status)
	require.False(t, present.ExpireAt.IsZero())
	require.WithinDuration(t, time.Now(), present.ExpireAt, time.Minute)

	// the gift must not touch the account balance and writes no statement
	var account database.AccountUser
	require.Nil(t, db.Core.NewSelect().Model(&account).Where("user_uuid = ?", userUUID).Scan(ctx))
	require.Equal(t, float64(77), account.Balance)
	statementCount, err := db.Core.NewSelect().Model((*database.AccountStatement)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, statementCount)

	// the canceled/expired record stays out of the legacy auto-expire scan
	autoExpirePresents, err := store.ListAutoExpirePresents(ctx, activityID, beginDate.Add(-time.Hour), endDate.Add(time.Hour), 0, 0, 100)
	require.Nil(t, err)
	require.Empty(t, autoExpirePresents)

	var vouchers []database.AccountVoucher
	require.Nil(t, db.Core.NewSelect().Model(&vouchers).Where("target_uuid = ?", userUUID).Order("id").Scan(ctx))
	require.Len(t, vouchers, 2)
	require.NotEmpty(t, vouchers[0].VoucherNo)
	require.NotEqual(t, vouchers[0].VoucherNo, vouchers[1].VoucherNo)
	require.Equal(t, types.VoucherTypeCompute, vouchers[0].VoucherType)
	require.Equal(t, types.VoucherTypeToken, vouchers[1].VoucherType)
	for _, voucher := range vouchers {
		require.Equal(t, float64(100), voucher.Total)
		require.Equal(t, types.VoucherStatusActive, voucher.Status)
		require.Equal(t, present.EventUUID.String(), voucher.BatchNo)
		require.WithinDuration(t, beginDate, voucher.BeginDate, time.Millisecond)
		require.WithinDuration(t, endDate, voucher.EndDate, time.Millisecond)
	}

	var sub database.AccountSubscription
	require.Nil(t, db.Core.NewSelect().Model(&sub).Where("user_uuid = ? and sku_type = ?", userUUID, types.SKUCSGLite).Scan(ctx))
	require.Equal(t, types.SubscriptionStatusActive, sub.Status)
	require.Equal(t, database.CSGLiteMonthlyResourceID, sub.ResourceID)
	require.NotZero(t, sub.PriceID)
	require.Zero(t, sub.NextPriceID)
	require.Empty(t, sub.NextResourceID)
	require.Zero(t, sub.AmountPaidTotal)
	require.Equal(t, int64(1), sub.AmountPaidCount)
	require.WithinDuration(t, time.Now(), sub.LastPeriodStart, time.Minute)
	require.WithinDuration(t, time.Now().AddDate(0, 1, 0), sub.LastPeriodEnd, 72*time.Hour)

	var bill database.AccountSubscriptionBill
	require.Nil(t, db.Core.NewSelect().Model(&bill).Where("sub_id = ?", sub.ID).Scan(ctx))
	require.Equal(t, sub.LastBillID, bill.ID)
	require.Equal(t, float64(0), bill.AmountPaid)
	require.Equal(t, types.BillingStatusPaid, bill.Status)
	require.Equal(t, types.BillingReasonSubscriptionCreate, bill.Reason)
	require.Equal(t, present.EventUUID.String(), bill.EventUUID)
	require.Equal(t, database.CSGLiteMonthlyResourceID, bill.ResourceID)
	require.WithinDuration(t, sub.LastPeriodEnd, bill.PeriodEnd, time.Second)

	// the gifted month issues the same CSGLite license a paid month does, so
	// the customer can download and activate it during the trial
	var license database.License
	licenseKey := fmt.Sprintf("subscription:%d:%d", types.SKUCSGLite, sub.ID)
	require.Nil(t, db.Core.NewSelect().Model(&license).Where("key = ?", licenseKey).Scan(ctx))
	require.Equal(t, userUUID, license.UserUUID)
	require.Equal(t, "new-user", license.Company)
	require.Equal(t, "new-user@example.com", license.Email)
	require.Equal(t, string(types.LicenseProductCSGLite), license.Product)
	require.Equal(t, "Enterprise", license.Edition)
	require.Equal(t, 1, license.MaxUser)
	require.Equal(t, "subscription", license.Issuer)
	require.WithinDuration(t, sub.LastPeriodStart, license.StartTime, time.Second)
	require.WithinDuration(t, sub.LastPeriodEnd, license.ExpireTime, time.Second)

	// a repeated send for the same user and activity is skipped untouched
	awarded, err = store.SendNewUserPresent(ctx, newNewUserPresentGift(userUUID, activityID, 100, beginDate, endDate))
	require.Nil(t, err)
	require.False(t, awarded)

	presentCount, err := db.Core.NewSelect().Model((*database.AccountPresent)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, presentCount)
	voucherCount, err := db.Core.NewSelect().Model((*database.AccountVoucher)(nil)).Where("target_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 2, voucherCount)
	subCount, err := db.Core.NewSelect().Model((*database.AccountSubscription)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, subCount)
	billCount, err := db.Core.NewSelect().Model((*database.AccountSubscriptionBill)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, billCount)
	licenseCount, err := db.Core.NewSelect().Model((*database.License)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, licenseCount)
	require.Equal(t, float64(77), account.Balance)
}

func TestAccountPresentStore_SendNewUserPresentRejectsNonPositiveVoucherValue(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewAccountPresentStoreWithDB(db)

	userUUID := "new-user-zero-voucher"
	activityID := int64(9106)
	now := time.Now()
	_, err := db.Core.NewInsert().Model(&database.AccountActivity{
		ActID:        activityID,
		ActCode:      fmt.Sprintf("test_activity_%d", activityID),
		ActName:      fmt.Sprintf("Test Activity %d", activityID),
		Status:       types.ActivityStatusActive,
		BeginAt:      now.Add(-time.Hour),
		EndAt:        now.Add(24 * time.Hour),
		VoucherValue: 0,
	}).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&database.AccountUser{UserUUID: userUUID, Balance: 10}).Exec(ctx)
	require.NoError(t, err)

	awarded, err := store.SendNewUserPresent(ctx, newNewUserPresentGift(userUUID, activityID, 100, now, now.AddDate(0, 1, 0)))
	require.ErrorIs(t, err, database.ErrActivityNotGrantable)
	require.False(t, awarded)

	presentCount, err := db.Core.NewSelect().Model((*database.AccountPresent)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, presentCount)
	voucherCount, err := db.Core.NewSelect().Model((*database.AccountVoucher)(nil)).Where("target_uuid = ?", userUUID).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, voucherCount)
}

func TestAccountPresentStore_SendNewUserPresentMissingAccount(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewAccountPresentStoreWithDB(db)

	userUUID := "missing-account-user"
	now := time.Now()
	awarded, err := store.SendNewUserPresent(ctx, newNewUserPresentGift(userUUID, 9102, 100, now, now.AddDate(0, 1, 0)))
	require.Error(t, err)
	require.False(t, awarded)

	presentCount, err := db.Core.NewSelect().Model((*database.AccountPresent)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, presentCount)
	voucherCount, err := db.Core.NewSelect().Model((*database.AccountVoucher)(nil)).Where("target_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, voucherCount)
}

func TestAccountPresentStore_SendNewUserPresentConcurrent(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAccountPresentStoreWithDB(db)

	userUUID := uuid.NewString()
	activityID := int64(9103)
	seedNewUserPresentActivity(t, ctx, db, activityID)
	_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: userUUID, Balance: 5}).Exec(ctx)
	require.NoError(t, err)

	results := make(chan bool, 8)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			now := time.Now()
			awarded, err := store.SendNewUserPresent(ctx, newNewUserPresentGift(userUUID, activityID, 100, now, now.AddDate(0, 1, 0)))
			results <- awarded
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	awardedCount := 0
	for awarded := range results {
		if awarded {
			awardedCount++
		}
	}
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 1, awardedCount)

	presentCount, err := db.Core.NewSelect().Model((*database.AccountPresent)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, presentCount)
	voucherCount, err := db.Core.NewSelect().Model((*database.AccountVoucher)(nil)).Where("target_uuid = ?", userUUID).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, voucherCount)
	subCount, err := db.Core.NewSelect().Model((*database.AccountSubscription)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, subCount)
}

func TestAccountPresentStore_SendNewUserPresentSkipsInactiveSubscription(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewAccountPresentStoreWithDB(db)

	userUUID := "gift-with-inactive-sub"
	seedNewUserPresentActivity(t, ctx, db, 9105)
	_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: userUUID, Balance: 10}).Exec(ctx)
	require.Nil(t, err)

	// an already ended CSGLite subscription still occupies the unique
	// (user_uuid, sku_type) row, so the gift must skip it instead of failing
	inactiveSub := database.AccountSubscription{
		UserUUID:        userUUID,
		SkuType:         types.SKUCSGLite,
		PriceID:         1,
		ResourceID:      database.CSGLiteMonthlyResourceID,
		Status:          types.SubscriptionStatusExpired,
		ActionUser:      userUUID,
		StartAt:         time.Now().AddDate(0, -2, 0),
		LastBillID:      987655,
		LastPeriodStart: time.Now().AddDate(0, -2, 0),
		LastPeriodEnd:   time.Now().AddDate(0, -1, 0),
		AmountPaidTotal: 3900,
		AmountPaidCount: 1,
	}
	_, err = db.Core.NewInsert().Model(&inactiveSub).Exec(ctx)
	require.Nil(t, err)

	now := time.Now()
	awarded, err := store.SendNewUserPresent(ctx, newNewUserPresentGift(userUUID, 9105, 100, now, now.AddDate(0, 1, 0)))
	require.Nil(t, err)
	require.True(t, awarded, "the present and vouchers are granted even when the subscription part is skipped")

	subCount, err := db.Core.NewSelect().Model((*database.AccountSubscription)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, subCount)
	var sub database.AccountSubscription
	require.Nil(t, db.Core.NewSelect().Model(&sub).Where("user_uuid = ?", userUUID).Scan(ctx))
	require.Equal(t, inactiveSub.ID, sub.ID)
	require.Equal(t, types.SubscriptionStatusExpired, sub.Status)
	require.Equal(t, int64(987655), sub.LastBillID)

	billCount, err := db.Core.NewSelect().Model((*database.AccountSubscriptionBill)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, billCount)

	// no gifted subscription means no gifted license either; a paid license
	// must not be shadowed
	licenseCount, err := db.Core.NewSelect().Model((*database.License)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, licenseCount)

	presentCount, err := db.Core.NewSelect().Model((*database.AccountPresent)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, presentCount)
	voucherCount, err := db.Core.NewSelect().Model((*database.AccountVoucher)(nil)).Where("target_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 2, voucherCount)
}

func TestAccountPresentStore_SendNewUserPresentKeepsLiveSubscription(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewAccountPresentStoreWithDB(db)

	userUUID := "gift-with-live-sub"
	seedNewUserPresentActivity(t, ctx, db, 9104)
	_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: userUUID, Balance: 10}).Exec(ctx)
	require.Nil(t, err)

	// a live paid CSGLite subscription that the gift must not shadow
	liveSub := database.AccountSubscription{
		UserUUID:        userUUID,
		SkuType:         types.SKUCSGLite,
		PriceID:         1,
		ResourceID:      database.CSGLiteMonthlyResourceID,
		Status:          types.SubscriptionStatusActive,
		ActionUser:      userUUID,
		StartAt:         time.Now(),
		LastBillID:      987654,
		LastPeriodStart: time.Now(),
		LastPeriodEnd:   time.Now().AddDate(0, 1, 0),
		AmountPaidTotal: 3900,
		AmountPaidCount: 1,
	}
	_, err = db.Core.NewInsert().Model(&liveSub).Exec(ctx)
	require.Nil(t, err)

	now := time.Now()
	awarded, err := store.SendNewUserPresent(ctx, newNewUserPresentGift(userUUID, 9104, 100, now, now.AddDate(0, 1, 0)))
	require.Nil(t, err)
	require.True(t, awarded, "the present and vouchers are granted even without a new subscription")

	subCount, err := db.Core.NewSelect().Model((*database.AccountSubscription)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, subCount)
	var sub database.AccountSubscription
	require.Nil(t, db.Core.NewSelect().Model(&sub).Where("user_uuid = ?", userUUID).Scan(ctx))
	require.Equal(t, liveSub.ID, sub.ID)
	require.Equal(t, float64(3900), sub.AmountPaidTotal)

	billCount, err := db.Core.NewSelect().Model((*database.AccountSubscriptionBill)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, billCount)

	// no gifted subscription means no gifted license either; a paid license
	// must not be shadowed
	licenseCount, err := db.Core.NewSelect().Model((*database.License)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, licenseCount)

	presentCount, err := db.Core.NewSelect().Model((*database.AccountPresent)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, presentCount)
	voucherCount, err := db.Core.NewSelect().Model((*database.AccountVoucher)(nil)).Where("target_uuid = ?", userUUID).Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 2, voucherCount)
}

func TestAccountPresentStore_SendNewUserPresentActivityWindow(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewAccountPresentStoreWithDB(db)
	now := time.Now()

	insertActivity := func(actID int64, status types.ActivityStatus, beginAt, endAt time.Time) {
		_, err := db.Core.NewInsert().Model(&database.AccountActivity{
			ActID:        actID,
			ActCode:      fmt.Sprintf("window_activity_%d", actID),
			ActName:      fmt.Sprintf("Window Activity %d", actID),
			Status:       status,
			BeginAt:      beginAt,
			EndAt:        endAt,
			VoucherValue: 100,
		}).Exec(ctx)
		require.NoError(t, err)
	}
	insertUser := func(userUUID string) {
		_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: userUUID, Balance: 10}).Exec(ctx)
		require.NoError(t, err)
	}

	cases := []struct {
		name    string
		actID   int64
		status  types.ActivityStatus
		beginAt time.Time
		endAt   time.Time
		wantErr bool
	}{
		{"missing activity", 9200, types.ActivityStatusActive, now.Add(-time.Hour), now.Add(time.Hour), true},
		{"inactive activity", 9201, types.ActivityStatusExpired, now.Add(-time.Hour), now.Add(time.Hour), true},
		{"zero begin time is an open lower bound", 9202, types.ActivityStatusActive, time.Time{}, now.Add(time.Hour), false},
		{"begin time in the future", 9203, types.ActivityStatusActive, now.Add(time.Hour), now.Add(2 * time.Hour), true},
		{"zero end time is an open upper bound", 9204, types.ActivityStatusActive, now.Add(-time.Hour), time.Time{}, false},
		{"ended activity", 9205, types.ActivityStatusActive, now.Add(-2 * time.Hour), now.Add(-time.Hour), true},
		{"inside the window", 9206, types.ActivityStatusActive, now.Add(-time.Hour), now.Add(time.Hour), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.actID != 9200 {
				insertActivity(tc.actID, tc.status, tc.beginAt, tc.endAt)
			}
			userUUID := uuid.NewString()
			insertUser(userUUID)

			awarded, err := store.SendNewUserPresent(ctx, newNewUserPresentGift(userUUID, tc.actID, 100, now, now.AddDate(0, 1, 0)))
			if tc.wantErr {
				require.ErrorIs(t, err, database.ErrActivityNotGrantable)
				require.False(t, awarded)
			} else {
				require.NoError(t, err)
				require.True(t, awarded)
			}

			presentCount, err := db.Core.NewSelect().Model((*database.AccountPresent)(nil)).Where("user_uuid = ?", userUUID).Count(ctx)
			require.NoError(t, err)
			voucherCount, err := db.Core.NewSelect().Model((*database.AccountVoucher)(nil)).Where("target_uuid = ?", userUUID).Count(ctx)
			require.NoError(t, err)
			if tc.wantErr {
				require.Zero(t, presentCount)
				require.Zero(t, voucherCount)
				return
			}
			require.Equal(t, 1, presentCount)
			require.Equal(t, 2, voucherCount)
		})
	}
}

// TestAccountPresentStore_SendNewUserPresentWaitsForSubscriptionLock proves the
// gift serializes with the paid purchase and renewal paths: those paths hold the
// per (sku, user) subscription advisory lock, and the gift must queue behind it
// instead of checking "no subscription yet" and inserting a second active row.
func TestAccountPresentStore_SendNewUserPresentWaitsForSubscriptionLock(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewAccountPresentStoreWithDB(db)

	userUUID := uuid.NewString()
	activityID := int64(9107)
	seedNewUserPresentActivity(t, ctx, db, activityID)
	_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: userUUID, Balance: 10}).Exec(ctx)
	require.NoError(t, err)

	// hold the subscription lock exactly like lockSubscriptionPurchase does
	lockHolder, err := db.BunDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = lockHolder.Rollback() }() // no-op once committed
	lockKey := fmt.Sprintf("subscription:%d:%s", types.SKUCSGLite, userUUID)
	_, err = lockHolder.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", lockKey)
	require.NoError(t, err)

	now := time.Now()
	done := make(chan error, 1)
	go func() {
		_, err := store.SendNewUserPresent(ctx, newNewUserPresentGift(userUUID, activityID, 100, now, now.AddDate(0, 1, 0)))
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("new user present did not wait for the subscription lock, error: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	require.NoError(t, lockHolder.Commit())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("new user present did not finish after the subscription lock was released")
	}

	// one row per (user, sku): a concurrent purchase can no longer be shadowed
	subs := []database.AccountSubscription{}
	require.NoError(t, db.Core.NewSelect().Model(&subs).
		Where("user_uuid = ?", userUUID).
		Where("sku_type = ?", types.SKUCSGLite).
		Scan(ctx))
	require.Len(t, subs, 1)
}
