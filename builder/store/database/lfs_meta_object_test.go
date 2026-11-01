package database_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

type xnetClaimQueryHook struct {
	query string
}

func (h *xnetClaimQueryHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if strings.Contains(event.Query, "JOIN repositories AS repository") && strings.Contains(event.Query, "FOR UPDATE") {
		h.query = event.Query
	}
	return ctx
}

func (*xnetClaimQueryHook) AfterQuery(context.Context, *bun.QueryEvent) {}

func createXnetIntentRepositories(t *testing.T, db *database.DB, repositoryIDs ...int64) {
	t.Helper()
	for _, repositoryID := range repositoryIDs {
		_, err := db.Core.NewInsert().Model(&database.Repository{
			ID: repositoryID, Path: fmt.Sprintf("xnet-intent/%d", repositoryID),
			GitPath: fmt.Sprintf("xnet-intent/%d.git", repositoryID), Name: fmt.Sprintf("repo-%d", repositoryID),
		}).Exec(context.Background())
		require.NoError(t, err)
	}
}

func activateXnetIntentRepositories(t *testing.T, db *database.DB, repositoryIDs ...int64) {
	t.Helper()
	createXnetIntentRepositories(t, db, repositoryIDs...)
	_, err := db.Core.NewUpdate().Model((*database.Repository)(nil)).
		Set("xnet_migration_intent = ?", database.XnetMigrationActive).
		Where("id IN (?)", bun.In(repositoryIDs)).Exec(context.Background())
	require.NoError(t, err)
}

func TestLfsMetaStore_ClaimXnetDispatchObjects(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	createXnetIntentRepositories(t, db, 1, 2, 3, 4)
	require.NoError(t, store.ResumeAllXnetDispatch(ctx, database.XnetDispatchResume{
		LeaseDuration: 10 * time.Minute, MaxAttempts: 2,
	}))
	now := time.Now()
	expired := now.Add(-11 * time.Minute)
	active := now.Add(-9 * time.Minute)

	objects := []database.LfsMetaObject{
		{RepositoryID: 1, Oid: "pending-1", XnetDispatchStatus: database.XnetDispatchPending},
		{RepositoryID: 1, Oid: "pending-2", XnetDispatchStatus: database.XnetDispatchPending},
		{RepositoryID: 2, Oid: "active", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &active, XnetDispatchAttempts: 1},
		{RepositoryID: 2, Oid: "expired", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &expired, XnetDispatchAttempts: 1},
		{RepositoryID: 3, Oid: "retryable", XnetDispatchStatus: database.XnetDispatchFailed, XnetDispatchAttempts: 1},
		{RepositoryID: 3, Oid: "exhausted", XnetDispatchStatus: database.XnetDispatchFailed, XnetDispatchAttempts: 2},
		{RepositoryID: 4, Oid: "completed", XnetUsed: true, XnetDispatchStatus: database.XnetDispatchCompleted},
	}
	for _, object := range objects {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: now, LeaseDuration: 10 * time.Minute, Limit: 3, MaxAttempts: 2,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 3)
	require.Equal(t, []string{"pending-1", "pending-2", "expired"}, []string{claimed[0].Oid, claimed[1].Oid, claimed[2].Oid})
	for _, object := range claimed {
		require.Equal(t, database.XnetDispatchDispatched, object.XnetDispatchStatus)
		require.WithinDuration(t, now, *object.XnetDispatchedAt, 5*time.Second)
		if object.Oid == "expired" {
			require.Equal(t, 2, object.XnetDispatchAttempts)
		} else {
			require.Equal(t, 1, object.XnetDispatchAttempts)
		}
	}

	second, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: now, LeaseDuration: 10 * time.Minute, Limit: 10, MaxAttempts: 2,
	})
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, "retryable", second[0].Oid)
	require.Equal(t, 2, second[0].XnetDispatchAttempts)
}

// Deleted repositories must not consume claim capacity or retry attempts while
// their LFS rows await asynchronous deletion.
func TestLfsMetaStore_ClaimExcludesSoftDeletedRepositories(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 9141, 9142)
	expired := time.Now().Add(-time.Hour)
	objects := []database.LfsMetaObject{
		{RepositoryID: 9141, Oid: "deleted-pending", XnetDispatchStatus: database.XnetDispatchPending},
		{RepositoryID: 9141, Oid: "deleted-retry", XnetDispatchStatus: database.XnetDispatchFailed, XnetDispatchAttempts: 1},
		{RepositoryID: 9141, Oid: "deleted-expired", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchAttempts: 1, XnetDispatchedAt: &expired},
		{RepositoryID: 9142, Oid: "live-pending", XnetDispatchStatus: database.XnetDispatchPending},
	}
	_, err := db.Core.NewInsert().Model(&objects).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewUpdate().Model((*database.Repository)(nil)).Set("deleted_at = NOW()").Where("id = ?", 9141).Exec(ctx)
	require.NoError(t, err)
	for _, repositoryID := range []*int64{nil, &objects[0].RepositoryID} {
		claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
			RepositoryID: repositoryID, LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 3,
		})
		require.NoError(t, err)
		if repositoryID == nil {
			require.Len(t, claimed, 1)
			require.Equal(t, "live-pending", claimed[0].Oid)
		} else {
			require.Empty(t, claimed)
		}
	}
	var unchanged []database.LfsMetaObject
	require.NoError(t, db.Core.NewSelect().Model(&unchanged).Where("repository_id = ?", 9141).Order("id ASC").Scan(ctx))
	require.Len(t, unchanged, 3)
	for i, object := range unchanged {
		require.Equal(t, objects[i].XnetDispatchStatus, object.XnetDispatchStatus)
		require.Equal(t, objects[i].XnetDispatchAttempts, object.XnetDispatchAttempts)
	}
}

func TestLfsMetaStore_RepositoryIntentScopesGlobalClaim(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	createXnetIntentRepositories(t, db, 9101, 9102)

	for _, object := range []database.LfsMetaObject{
		{RepositoryID: 9101, Oid: "active-repository"},
		{RepositoryID: 9102, Oid: "inactive-repository"},
	} {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}
	require.NoError(t, store.ResumeXnetDispatch(ctx, database.XnetDispatchResume{
		LeaseDuration: 10 * time.Minute, MaxAttempts: 3, RepositoryID: 9101,
	}))

	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		LeaseDuration: 10 * time.Minute, Limit: 10, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, "active-repository", claimed[0].Oid)
}

func TestLfsMetaStore_ClaimQualifiesJoinedOrderAndLockTarget(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 9151)
	_, err := store.Create(ctx, database.LfsMetaObject{RepositoryID: 9151, Oid: "qualified-claim"})
	require.NoError(t, err)
	hook := &xnetClaimQueryHook{}
	db.BunDB.AddQueryHook(hook)

	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Contains(t, hook.query, `ORDER BY "lfs_meta_object"."id" ASC`)
	require.Contains(t, hook.query, `FOR UPDATE OF lfs_meta_object SKIP LOCKED`)
}

func TestLfsMetaStore_RepositoryPauseExcludesFutureObjects(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	createXnetIntentRepositories(t, db, 9201)

	require.NoError(t, store.ResumeXnetDispatch(ctx, database.XnetDispatchResume{
		LeaseDuration: 10 * time.Minute, MaxAttempts: 3, RepositoryID: 9201,
	}))
	require.NoError(t, pauseXnetDispatchForTest(ctx, db, 9201))
	_, err := store.Create(ctx, database.LfsMetaObject{RepositoryID: 9201, Oid: "created-after-repository-stop"})
	require.NoError(t, err)

	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		LeaseDuration: 10 * time.Minute, Limit: 10, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Empty(t, claimed)
}

func TestLfsMetaStore_StopAllExcludesFutureObjects(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	createXnetIntentRepositories(t, db, 9301)

	require.NoError(t, store.ResumeXnetDispatch(ctx, database.XnetDispatchResume{
		LeaseDuration: 10 * time.Minute, MaxAttempts: 3, RepositoryID: 9301,
	}))
	require.NoError(t, database.NewXnetDispatchFenceWithDB(db).WithXnetDispatchStopFence(ctx, nil, func() error { return nil }))
	_, err := store.Create(ctx, database.LfsMetaObject{RepositoryID: 9301, Oid: "created-after-stop-all"})
	require.NoError(t, err)

	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		LeaseDuration: 10 * time.Minute, Limit: 10, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Empty(t, claimed)
}

func TestLfsMetaStore_ExpireExhaustedXnetDispatchesReturnsOnlyNewlyExpiredRepositories(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)

	now := time.Now()
	expired := now.Add(-2 * time.Minute)
	active := now
	objects := []database.LfsMetaObject{
		{RepositoryID: 7, Oid: "expired-a", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &expired, XnetDispatchAttempts: 2},
		{RepositoryID: 7, Oid: "expired-b", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &expired, XnetDispatchAttempts: 2},
		{RepositoryID: 8, Oid: "active", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &active, XnetDispatchAttempts: 2},
		{RepositoryID: 9, Oid: "retryable", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &expired, XnetDispatchAttempts: 1},
	}
	for _, object := range objects {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	repositories, err := store.ExpireExhaustedXnetDispatches(ctx, database.XnetDispatchExpiry{
		LeaseDuration: time.Minute,
		MaxAttempts:   2,
		Limit:         2,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{7}, repositories)

	// The transition is one-shot, so an idle heartbeat does not repeatedly sync
	// repositories that were exhausted by an earlier batch.
	repositories, err = store.ExpireExhaustedXnetDispatches(ctx, database.XnetDispatchExpiry{
		LeaseDuration: time.Minute,
		MaxAttempts:   2,
		Limit:         2,
	})
	require.NoError(t, err)
	require.Empty(t, repositories)
}

func TestLfsMetaStore_XnetDispatchIgnoresSkewedCallerClock(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 1)

	_, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID: 1,
		Oid:          "clock-skew",
	})
	require.NoError(t, err)

	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: time.Now().Add(-24 * time.Hour), LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	// A replica whose clock is far ahead must not consider the database lease expired.
	claimed, err = store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: time.Now().Add(24 * time.Hour), LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Empty(t, claimed)

	stats, err := store.GetXnetDispatchStats(ctx, database.XnetDispatchStatsQuery{
		Now: time.Now().Add(24 * time.Hour), LeaseDuration: 10 * time.Minute, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Equal(t, 1, stats.Active)
	require.Zero(t, stats.Retryable)
}

func TestLfsMetaStore_CompleteXnetDispatchIsIdempotent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	dispatchedAt := time.Now().Add(-time.Minute)

	_, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID: 123, Oid: "complete-me", XnetDispatchStatus: database.XnetDispatchDispatched,
		XnetDispatchedAt: &dispatchedAt, XnetDispatchAttempts: 1,
	})
	require.NoError(t, err)

	for range 2 {
		err = store.CompleteXnetDispatch(ctx, 123, "complete-me")
		require.NoError(t, err)
	}

	object, err := store.FindByOID(ctx, 123, "complete-me")
	require.NoError(t, err)
	require.True(t, object.XnetUsed)
	require.Equal(t, database.XnetDispatchCompleted, object.XnetDispatchStatus)
	require.Nil(t, object.XnetDispatchedAt)
	require.Equal(t, 1, object.XnetDispatchAttempts)
}

func TestLfsMetaStore_XnetDispatchStatsAndFailure(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	now := time.Now()
	active := now.Add(-time.Minute)
	expired := now.Add(-20 * time.Minute)

	objects := []database.LfsMetaObject{
		{RepositoryID: 1, Oid: "pending", XnetDispatchStatus: database.XnetDispatchPending},
		{RepositoryID: 1, Oid: "active", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &active, XnetDispatchAttempts: 1},
		{RepositoryID: 1, Oid: "expired", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &expired, XnetDispatchAttempts: 1},
		{RepositoryID: 1, Oid: "retryable", XnetDispatchStatus: database.XnetDispatchFailed, XnetDispatchAttempts: 1},
		{RepositoryID: 1, Oid: "exhausted", XnetDispatchStatus: database.XnetDispatchFailed, XnetDispatchAttempts: 2},
		{RepositoryID: 1, Oid: "completed", XnetUsed: true, XnetDispatchStatus: database.XnetDispatchCompleted},
	}
	for _, object := range objects {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}
	activeClaim, err := store.FindByOID(ctx, 1, "active")
	require.NoError(t, err)

	stats, err := store.GetXnetDispatchStats(ctx, database.XnetDispatchStatsQuery{
		Now: now, LeaseDuration: 10 * time.Minute, MaxAttempts: 2,
	})
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchStats{
		Active: 1, Retryable: 3, Exhausted: 1, Completed: 1,
		ExhaustedByRepository: map[int64]int{1: 1},
	}, stats)

	_, err = store.FailXnetDispatch(ctx, *activeClaim)
	require.NoError(t, err)
	failed, err := store.FindByOID(ctx, 1, "active")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchFailed, failed.XnetDispatchStatus)
	require.Nil(t, failed.XnetDispatchedAt)
}

func TestLfsMetaStore_FailedDispatchWaitsForBackoff(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 1)
	claimAt := time.Now()

	created, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID: 1,
		Oid:          "retry-with-backoff",
	})
	require.NoError(t, err)
	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: claimAt, LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 5,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, 1, claimed[0].XnetDispatchAttempts)

	_, err = store.FailXnetDispatch(ctx, claimed[0])
	require.NoError(t, err)
	failed, err := store.FindByOID(ctx, 1, created.Oid)
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchFailed, failed.XnetDispatchStatus)
	require.NotNil(t, failed.XnetNextAttemptAt)

	claimed, err = store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now:           failed.XnetNextAttemptAt.Add(-time.Nanosecond),
		LeaseDuration: 10 * time.Minute,
		Limit:         1,
		MaxAttempts:   5,
	})
	require.NoError(t, err)
	require.Empty(t, claimed)

	claimed, err = store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		LeaseDuration: 10 * time.Minute,
		Limit:         1,
		MaxAttempts:   5,
	})
	require.NoError(t, err)
	require.Empty(t, claimed)
	_, err = db.Operator.Core.NewUpdate().Model((*database.LfsMetaObject)(nil)).
		Set("xnet_next_attempt_at = clock_timestamp() - INTERVAL '1 microsecond'").
		Where("id = ?", created.ID).Exec(ctx)
	require.NoError(t, err)
	claimed, err = store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		LeaseDuration: 10 * time.Minute,
		Limit:         1,
		MaxAttempts:   5,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, 2, claimed[0].XnetDispatchAttempts)
}

func TestLfsMetaStore_FailureBackoffIncreasesWithAttempts(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	dispatchedAt := time.Now()

	created, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID:         1,
		Oid:                  "exponential-backoff",
		XnetDispatchStatus:   database.XnetDispatchDispatched,
		XnetDispatchAttempts: 2,
		XnetDispatchedAt:     &dispatchedAt,
	})
	require.NoError(t, err)
	created, err = store.FindByOID(ctx, created.RepositoryID, created.Oid)
	require.NoError(t, err)
	failureRecordedAt := time.Now()
	_, err = store.FailXnetDispatch(ctx, *created)
	require.NoError(t, err)

	failed, err := store.FindByOID(ctx, 1, created.Oid)
	require.NoError(t, err)
	require.NotNil(t, failed.XnetNextAttemptAt)
	require.GreaterOrEqual(t, failed.XnetNextAttemptAt.Sub(failureRecordedAt), 2*time.Minute-time.Second)
	require.Less(t, failed.XnetNextAttemptAt.Sub(failureRecordedAt), 2*time.Minute+time.Second)
}

func TestLfsMetaStore_FailureBackoffIsCappedForLargeAttemptCount(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	dispatchedAt := time.Now()

	created, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID:         1,
		Oid:                  "capped-backoff",
		XnetDispatchStatus:   database.XnetDispatchDispatched,
		XnetDispatchAttempts: 100,
		XnetDispatchedAt:     &dispatchedAt,
	})
	require.NoError(t, err)
	created, err = store.FindByOID(ctx, created.RepositoryID, created.Oid)
	require.NoError(t, err)
	failureRecordedAt := time.Now()
	_, err = store.FailXnetDispatch(ctx, *created)
	require.NoError(t, err)

	failed, err := store.FindByOID(ctx, 1, created.Oid)
	require.NoError(t, err)
	require.NotNil(t, failed.XnetNextAttemptAt)
	require.GreaterOrEqual(t, failed.XnetNextAttemptAt.Sub(failureRecordedAt), 30*time.Minute-time.Second)
	require.Less(t, failed.XnetNextAttemptAt.Sub(failureRecordedAt), 30*time.Minute+time.Second)
}

func TestLfsMetaStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewLfsMetaObjectStoreWithDB(db)
	_, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID: 123,
		Oid:          "foobar",
	})
	require.Nil(t, err)

	obj := &database.LfsMetaObject{}
	err = db.Core.NewSelect().Model(obj).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foobar", obj.Oid)

	obj, err = store.FindByOID(ctx, 123, "foobar")
	require.Nil(t, err)
	require.Equal(t, "foobar", obj.Oid)

	objs, err := store.FindByRepoID(ctx, 123)
	require.Nil(t, err)
	require.Equal(t, 1, len(objs))
	require.Equal(t, "foobar", objs[0].Oid)

	objs, err = store.FindByRepoID(ctx, 987)
	require.Nil(t, err)
	require.Equal(t, 0, len(objs))

	// update
	_, err = store.UpdateOrCreate(ctx, database.LfsMetaObject{
		RepositoryID: 123,
		Oid:          "foobar",
		Size:         999,
	})
	require.Nil(t, err)
	obj, err = store.FindByOID(ctx, 123, "foobar")
	require.Nil(t, err)
	require.Equal(t, 999, int(obj.Size))

	// create
	_, err = store.UpdateOrCreate(ctx, database.LfsMetaObject{
		RepositoryID: 456,
		Oid:          "bar",
		Size:         998,
	})
	require.Nil(t, err)
	obj, err = store.FindByOID(ctx, 456, "bar")
	require.Nil(t, err)
	require.Equal(t, 998, int(obj.Size))

	err = store.BulkUpdateOrCreate(ctx, int64(456), []database.LfsMetaObject{
		{RepositoryID: 456, Oid: "foobar", Size: 1},
		{RepositoryID: 456, Oid: "bar", Size: 2},
		{RepositoryID: 456, Oid: "barfoo", Size: 3},
	})
	require.Nil(t, err)

	obj, err = store.FindByOID(ctx, 456, "foobar")
	require.Nil(t, err)
	require.Equal(t, 1, int(obj.Size))
	obj, err = store.FindByOID(ctx, 456, "bar")
	require.Nil(t, err)
	require.Equal(t, 2, int(obj.Size))
	obj, err = store.FindByOID(ctx, 456, "barfoo")
	require.Nil(t, err)
	require.Equal(t, 3, int(obj.Size))

	err = store.RemoveByOid(ctx, "foobar", 456)
	require.Nil(t, err)
	_, err = store.FindByOID(ctx, 456, "foobar")
	require.NotNil(t, err)

}

func TestLfsMetaStore_UpdateOrCreatePreservesXnetDispatchState(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	dispatchedAt := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
	nextAttemptAt := dispatchedAt.Add(4 * time.Minute)

	_, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID:         123,
		Oid:                  "preserve-on-update",
		Size:                 1,
		Existing:             false,
		XnetDispatchStatus:   database.XnetDispatchFailed,
		XnetDispatchedAt:     &dispatchedAt,
		XnetNextAttemptAt:    &nextAttemptAt,
		XnetDispatchAttempts: 3,
	})
	require.NoError(t, err)

	updated, err := store.UpdateOrCreate(ctx, database.LfsMetaObject{
		RepositoryID: 123,
		Oid:          "preserve-on-update",
		Size:         99,
		Existing:     true,
	})
	require.NoError(t, err)
	require.Equal(t, int64(99), updated.Size)
	require.True(t, updated.Existing)
	require.Equal(t, database.XnetDispatchFailed, updated.XnetDispatchStatus)
	require.Equal(t, &dispatchedAt, updated.XnetDispatchedAt)
	require.Equal(t, &nextAttemptAt, updated.XnetNextAttemptAt)
	require.Equal(t, 3, updated.XnetDispatchAttempts)
}

func TestLfsMetaStore_BulkUpdateOrCreatePreservesXnetDispatchState(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	dispatchedAt := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)

	_, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID:         456,
		Oid:                  "preserve-in-bulk",
		Size:                 1,
		XnetDispatchStatus:   database.XnetDispatchDispatched,
		XnetDispatchedAt:     &dispatchedAt,
		XnetDispatchAttempts: 2,
	})
	require.NoError(t, err)

	err = store.BulkUpdateOrCreate(ctx, 456, []database.LfsMetaObject{
		{RepositoryID: 456, Oid: "preserve-in-bulk", Size: 99, Existing: true},
		{RepositoryID: 456, Oid: "new-in-bulk", Size: 5},
	})
	require.NoError(t, err)

	preserved, err := store.FindByOID(ctx, 456, "preserve-in-bulk")
	require.NoError(t, err)
	require.Equal(t, int64(99), preserved.Size)
	require.True(t, preserved.Existing)
	require.Equal(t, database.XnetDispatchDispatched, preserved.XnetDispatchStatus)
	require.Equal(t, &dispatchedAt, preserved.XnetDispatchedAt)
	require.Equal(t, 2, preserved.XnetDispatchAttempts)

	created, err := store.FindByOID(ctx, 456, "new-in-bulk")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPending, created.XnetDispatchStatus)
	require.Zero(t, created.XnetDispatchAttempts)
}

func TestLfsMetaStore_PauseAndResumeXnetDispatchByRepository(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 1, 2)
	now := time.Now()
	nextAttemptAt := now.Add(time.Hour)

	objects := []database.LfsMetaObject{
		{RepositoryID: 1, Oid: "repo-1-pending", XnetDispatchStatus: database.XnetDispatchPending},
		{RepositoryID: 1, Oid: "repo-1-partially-used-budget", XnetDispatchStatus: database.XnetDispatchPending, XnetDispatchAttempts: 2},
		{RepositoryID: 1, Oid: "repo-1-failed", XnetDispatchStatus: database.XnetDispatchFailed, XnetNextAttemptAt: &nextAttemptAt, XnetDispatchAttempts: 5},
		{RepositoryID: 1, Oid: "repo-1-completed", XnetUsed: true, XnetDispatchStatus: database.XnetDispatchCompleted},
		{RepositoryID: 2, Oid: "repo-2-pending", XnetDispatchStatus: database.XnetDispatchPending},
	}
	for _, object := range objects {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	require.NoError(t, pauseXnetDispatchForTest(ctx, db, 1))
	paused, err := store.FindByRepoID(ctx, 1)
	require.NoError(t, err)
	require.Len(t, paused, 4)
	for _, object := range paused {
		if object.XnetUsed {
			require.Equal(t, database.XnetDispatchCompleted, object.XnetDispatchStatus)
			continue
		}
		require.Equal(t, database.XnetDispatchPaused, object.XnetDispatchStatus)
		require.Nil(t, object.XnetDispatchedAt)
		require.Nil(t, object.XnetNextAttemptAt)
	}

	stats, err := store.GetXnetDispatchStats(ctx, database.XnetDispatchStatsQuery{
		Now: now, LeaseDuration: 10 * time.Minute, MaxAttempts: 5,
	})
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchStats{Retryable: 1, Completed: 1}, stats)
	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: now, LeaseDuration: 10 * time.Minute, Limit: 10, MaxAttempts: 5,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, "repo-2-pending", claimed[0].Oid)

	require.NoError(t, store.ResumeXnetDispatch(ctx, database.XnetDispatchResume{
		Now: now, LeaseDuration: 10 * time.Minute, MaxAttempts: 5, RepositoryID: 1,
	}))
	resumed, err := store.FindByRepoID(ctx, 1)
	require.NoError(t, err)
	for _, object := range resumed {
		if !object.XnetUsed {
			require.Equal(t, database.XnetDispatchPending, object.XnetDispatchStatus)
			if object.Oid == "repo-1-failed" {
				require.Zero(t, object.XnetDispatchAttempts)
			}
			if object.Oid == "repo-1-partially-used-budget" {
				require.Equal(t, 2, object.XnetDispatchAttempts)
			}
		}
	}
	repositoryID := int64(1)
	claimed, err = store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: now, LeaseDuration: 10 * time.Minute, Limit: 10, MaxAttempts: 5, RepositoryID: &repositoryID,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 3)
}

func TestLfsMetaStore_PauseAndResumeAllXnetDispatch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 1, 2)
	now := time.Now()

	for _, object := range []database.LfsMetaObject{
		{RepositoryID: 1, Oid: "repo-1", XnetDispatchAttempts: 2},
		{RepositoryID: 2, Oid: "repo-2"},
	} {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	require.NoError(t, database.NewXnetDispatchFenceWithDB(db).WithXnetDispatchStopFence(ctx, nil, func() error { return nil }))
	for _, repoID := range []int64{1, 2} {
		objects, err := store.FindByRepoID(ctx, repoID)
		require.NoError(t, err)
		require.Equal(t, database.XnetDispatchPending, objects[0].XnetDispatchStatus)
	}
	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: now, LeaseDuration: 10 * time.Minute, Limit: 10, MaxAttempts: 5,
	})
	require.NoError(t, err)
	require.Empty(t, claimed)

	require.NoError(t, store.ResumeAllXnetDispatch(ctx, database.XnetDispatchResume{
		Now: now, LeaseDuration: 10 * time.Minute, MaxAttempts: 5,
	}))
	for _, repoID := range []int64{1, 2} {
		objects, err := store.FindByRepoID(ctx, repoID)
		require.NoError(t, err)
		require.Equal(t, database.XnetDispatchPending, objects[0].XnetDispatchStatus)
		if repoID == 1 {
			require.Equal(t, 2, objects[0].XnetDispatchAttempts)
		}
	}
}

func TestLfsMetaStore_ResumeAllRestoresBudgetForExhaustedGloballyPausedObject(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 1)
	now := time.Now()

	object, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID: 1, Oid: "exhausted-before-global-pause",
		XnetDispatchStatus: database.XnetDispatchFailed, XnetDispatchAttempts: 5,
	})
	require.NoError(t, err)
	require.NoError(t, database.NewXnetDispatchFenceWithDB(db).WithXnetDispatchStopFence(ctx, nil, func() error { return nil }))
	require.NoError(t, store.ResumeAllXnetDispatch(ctx, database.XnetDispatchResume{
		Now: now, LeaseDuration: 10 * time.Minute, MaxAttempts: 5,
	}))

	resumed, err := store.FindByOID(ctx, object.RepositoryID, object.Oid)
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPending, resumed.XnetDispatchStatus)
	require.Zero(t, resumed.XnetDispatchAttempts)
	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: now, LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 5,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, object.Oid, claimed[0].Oid)
}

func TestLfsMetaStore_ResumeXnetDispatchRestartsFailedObjects(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	nextAttemptAt := time.Now().Add(time.Hour)

	for _, object := range []database.LfsMetaObject{
		{
			RepositoryID:         1,
			Oid:                  "exhausted",
			XnetDispatchStatus:   database.XnetDispatchFailed,
			XnetDispatchAttempts: 5,
			XnetNextAttemptAt:    &nextAttemptAt,
		},
		{
			RepositoryID:         2,
			Oid:                  "other-repository",
			XnetDispatchStatus:   database.XnetDispatchFailed,
			XnetDispatchAttempts: 5,
			XnetNextAttemptAt:    &nextAttemptAt,
		},
	} {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	require.NoError(t, store.ResumeXnetDispatch(ctx, database.XnetDispatchResume{
		Now: time.Now(), LeaseDuration: 10 * time.Minute, MaxAttempts: 5, RepositoryID: 1,
	}))

	restarted, err := store.FindByOID(ctx, 1, "exhausted")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPending, restarted.XnetDispatchStatus)
	require.Zero(t, restarted.XnetDispatchAttempts)
	require.Nil(t, restarted.XnetDispatchedAt)
	require.Nil(t, restarted.XnetNextAttemptAt)

	untouched, err := store.FindByOID(ctx, 2, "other-repository")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchFailed, untouched.XnetDispatchStatus)
	require.Equal(t, 5, untouched.XnetDispatchAttempts)
	require.NotNil(t, untouched.XnetNextAttemptAt)
}

func TestLfsMetaStore_ResumeXnetDispatchRestartsExpiredExhaustedDispatch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	now := time.Now()
	expiredAt := now.Add(-11 * time.Minute)
	activeAt := now.Add(-time.Minute)

	for _, object := range []database.LfsMetaObject{
		{RepositoryID: 1, Oid: "expired-exhausted", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchAttempts: 5, XnetDispatchedAt: &expiredAt},
		{RepositoryID: 1, Oid: "active-exhausted", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchAttempts: 5, XnetDispatchedAt: &activeAt},
		{RepositoryID: 2, Oid: "other-repository", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchAttempts: 5, XnetDispatchedAt: &expiredAt},
	} {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	require.NoError(t, store.ResumeXnetDispatch(ctx, database.XnetDispatchResume{
		Now: now, LeaseDuration: 10 * time.Minute, MaxAttempts: 5, RepositoryID: 1,
	}))

	restarted, err := store.FindByOID(ctx, 1, "expired-exhausted")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPending, restarted.XnetDispatchStatus)
	require.Zero(t, restarted.XnetDispatchAttempts)
	require.Nil(t, restarted.XnetDispatchedAt)

	active, err := store.FindByOID(ctx, 1, "active-exhausted")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchDispatched, active.XnetDispatchStatus)
	require.Equal(t, 5, active.XnetDispatchAttempts)

	other, err := store.FindByOID(ctx, 2, "other-repository")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchDispatched, other.XnetDispatchStatus)
}

func TestLfsMetaStore_ResumeAllRestartsFailedButPreservesRepositoryPause(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	createXnetIntentRepositories(t, db, 1, 2, 3)
	_, err := db.Core.NewUpdate().Model((*database.Repository)(nil)).
		Set("xnet_migration_intent = ?", database.XnetMigrationPaused).Where("id = ?", 2).Exec(ctx)
	require.NoError(t, err)
	nextAttemptAt := time.Now().Add(time.Hour)

	for _, object := range []database.LfsMetaObject{
		{
			RepositoryID:         1,
			Oid:                  "exhausted",
			XnetDispatchStatus:   database.XnetDispatchFailed,
			XnetDispatchAttempts: 5,
			XnetNextAttemptAt:    &nextAttemptAt,
		},
		{RepositoryID: 2, Oid: "repository-paused", XnetDispatchStatus: database.XnetDispatchPaused},
		{RepositoryID: 3, Oid: "globally-paused", XnetDispatchStatus: database.XnetDispatchGlobalPause},
	} {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	require.NoError(t, store.ResumeAllXnetDispatch(ctx, database.XnetDispatchResume{
		Now: time.Now(), LeaseDuration: 10 * time.Minute, MaxAttempts: 5,
	}))

	restarted, err := store.FindByOID(ctx, 1, "exhausted")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPending, restarted.XnetDispatchStatus)
	require.Zero(t, restarted.XnetDispatchAttempts)
	require.Nil(t, restarted.XnetNextAttemptAt)

	manuallyPaused, err := store.FindByOID(ctx, 2, "repository-paused")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPaused, manuallyPaused.XnetDispatchStatus)

	globallyPaused, err := store.FindByOID(ctx, 3, "globally-paused")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPending, globallyPaused.XnetDispatchStatus)
}

func TestLfsMetaStore_ResumeAllRestartsExpiredExhaustedDispatchButPreservesRepositoryPause(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	createXnetIntentRepositories(t, db, 1, 2)
	_, err := db.Core.NewUpdate().Model((*database.Repository)(nil)).
		Set("xnet_migration_intent = ?", database.XnetMigrationPaused).Where("id = ?", 2).Exec(ctx)
	require.NoError(t, err)
	now := time.Now()
	expiredAt := now.Add(-11 * time.Minute)

	for _, object := range []database.LfsMetaObject{
		{RepositoryID: 1, Oid: "expired-exhausted", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchAttempts: 5, XnetDispatchedAt: &expiredAt},
		{RepositoryID: 2, Oid: "repository-paused", XnetDispatchStatus: database.XnetDispatchPaused, XnetDispatchAttempts: 5},
	} {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	require.NoError(t, store.ResumeAllXnetDispatch(ctx, database.XnetDispatchResume{
		Now: now, LeaseDuration: 10 * time.Minute, MaxAttempts: 5,
	}))

	restarted, err := store.FindByOID(ctx, 1, "expired-exhausted")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPending, restarted.XnetDispatchStatus)
	require.Zero(t, restarted.XnetDispatchAttempts)

	paused, err := store.FindByOID(ctx, 2, "repository-paused")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPaused, paused.XnetDispatchStatus)
}

func TestLfsMetaStore_ResumeAllDoesNotResumeRepositoryPause(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 1, 2)

	for _, object := range []database.LfsMetaObject{
		{RepositoryID: 1, Oid: "manually-paused"},
		{RepositoryID: 2, Oid: "globally-paused"},
	} {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	require.NoError(t, pauseXnetDispatchForTest(ctx, db, 1))
	require.NoError(t, database.NewXnetDispatchFenceWithDB(db).WithXnetDispatchStopFence(ctx, nil, func() error { return nil }))
	require.NoError(t, store.ResumeAllXnetDispatch(ctx, database.XnetDispatchResume{
		Now: time.Now(), LeaseDuration: 10 * time.Minute, MaxAttempts: 5,
	}))

	manual, err := store.FindByOID(ctx, 1, "manually-paused")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPaused, manual.XnetDispatchStatus)
	global, err := store.FindByOID(ctx, 2, "globally-paused")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPending, global.XnetDispatchStatus)
}

func TestLfsMetaStore_RepositoryPauseOverridesGlobalPause(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)

	object, err := store.Create(ctx, database.LfsMetaObject{RepositoryID: 1, Oid: "repository-paused-after-global"})
	require.NoError(t, err)
	require.NoError(t, database.NewXnetDispatchFenceWithDB(db).WithXnetDispatchStopFence(ctx, nil, func() error { return nil }))
	require.NoError(t, pauseXnetDispatchForTest(ctx, db, object.RepositoryID))
	require.NoError(t, store.ResumeAllXnetDispatch(ctx, database.XnetDispatchResume{
		Now: time.Now(), LeaseDuration: 10 * time.Minute, MaxAttempts: 5,
	}))

	paused, err := store.FindByOID(ctx, object.RepositoryID, object.Oid)
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPaused, paused.XnetDispatchStatus)
}

func TestLfsMetaStore_StopFenceRepositoryPauseOverridesGlobalPause(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	fence := database.NewXnetDispatchFenceWithDB(db)

	object, err := store.Create(ctx, database.LfsMetaObject{RepositoryID: 1, Oid: "fenced-repository-paused-after-global"})
	require.NoError(t, err)
	require.NoError(t, fence.WithXnetDispatchStopFence(ctx, nil, func() error { return nil }))
	require.NoError(t, fence.WithXnetDispatchStopFence(ctx, &object.RepositoryID, func() error { return nil }))
	require.NoError(t, store.ResumeAllXnetDispatch(ctx, database.XnetDispatchResume{
		Now: time.Now(), LeaseDuration: 10 * time.Minute, MaxAttempts: 5,
	}))

	paused, err := store.FindByOID(ctx, object.RepositoryID, object.Oid)
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPaused, paused.XnetDispatchStatus)
}

func TestLfsMetaStore_ClaimXnetDispatchObjectsForRepository(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 1, 2)
	now := time.Now()
	repoID := int64(2)

	for _, object := range []database.LfsMetaObject{
		{RepositoryID: 1, Oid: "other-repo"},
		{RepositoryID: repoID, Oid: "target-1"},
		{RepositoryID: repoID, Oid: "target-2"},
	} {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: now, LeaseDuration: 10 * time.Minute, Limit: 10, MaxAttempts: 3, RepositoryID: &repoID,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 2)
	require.Equal(t, []string{"target-1", "target-2"}, []string{claimed[0].Oid, claimed[1].Oid})

	other, err := store.FindByOID(ctx, 1, "other-repo")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPending, other.XnetDispatchStatus)
}

func TestLfsMetaStore_ClaimXnetDispatchObjectsHonorsGlobalMaxInFlight(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 1, 2)
	now := time.Now()
	activeAt := now.Add(-time.Minute)
	repoID := int64(2)

	for _, object := range []database.LfsMetaObject{
		{RepositoryID: 1, Oid: "active-elsewhere", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &activeAt, XnetDispatchAttempts: 1},
		{RepositoryID: repoID, Oid: "target-1"},
		{RepositoryID: repoID, Oid: "target-2"},
	} {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: now, LeaseDuration: 10 * time.Minute, Limit: 10, MaxInFlight: 2, MaxAttempts: 3, RepositoryID: &repoID,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	claimed, err = store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: now, LeaseDuration: 10 * time.Minute, Limit: 10, MaxInFlight: 2, MaxAttempts: 3, RepositoryID: &repoID,
	})
	require.NoError(t, err)
	require.Empty(t, claimed)
}

func TestLfsMetaStore_GetXnetDispatchStatsScopesRepository(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	now := time.Now()
	activeAt := now.Add(-time.Minute)
	repoID := int64(1)

	for _, object := range []database.LfsMetaObject{
		{RepositoryID: repoID, Oid: "target-pending"},
		{RepositoryID: repoID, Oid: "target-completed", XnetUsed: true, XnetDispatchStatus: database.XnetDispatchCompleted},
		{RepositoryID: 2, Oid: "other-active", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &activeAt, XnetDispatchAttempts: 1},
		{RepositoryID: 2, Oid: "other-pending"},
	} {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	stats, err := store.GetXnetDispatchStats(ctx, database.XnetDispatchStatsQuery{
		Now: now, LeaseDuration: 10 * time.Minute, MaxAttempts: 3, RepositoryID: &repoID,
	})
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchStats{Retryable: 1, Completed: 1}, stats)
}

func TestLfsMetaStore_GetXnetDispatchStatsGroupsExhaustedByRepository(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	now := time.Now()

	for _, object := range []database.LfsMetaObject{
		{RepositoryID: 1, Oid: "repo-1-exhausted-1", XnetDispatchStatus: database.XnetDispatchFailed, XnetDispatchAttempts: 3},
		{RepositoryID: 1, Oid: "repo-1-exhausted-2", XnetDispatchStatus: database.XnetDispatchFailed, XnetDispatchAttempts: 4},
		{RepositoryID: 2, Oid: "repo-2-exhausted", XnetDispatchStatus: database.XnetDispatchFailed, XnetDispatchAttempts: 3},
		{RepositoryID: 2, Oid: "repo-2-retryable", XnetDispatchStatus: database.XnetDispatchFailed, XnetDispatchAttempts: 2},
		{RepositoryID: 3, Oid: "repo-3-paused", XnetDispatchStatus: database.XnetDispatchPaused, XnetDispatchAttempts: 3},
	} {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	stats, err := store.GetXnetDispatchStats(ctx, database.XnetDispatchStatsQuery{
		Now: now, LeaseDuration: 10 * time.Minute, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Equal(t, 3, stats.Exhausted)
	require.Equal(t, map[int64]int{1: 2, 2: 1}, stats.ExhaustedByRepository)
}

func TestLfsMetaStore_XnetDispatchClaimFenceRejectsPausedAndStaleClaims(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 1)
	fence := database.NewXnetDispatchFenceWithDB(db)
	firstClaimAt := time.Now()

	_, err := store.Create(ctx, database.LfsMetaObject{RepositoryID: 1, Oid: "fenced"})
	require.NoError(t, err)
	first, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: firstClaimAt, LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Len(t, first, 1)
	publishCount := 0
	published, err := fence.WithXnetDispatchPublishFence(ctx, first[0], func() error {
		publishCount++
		return nil
	})
	require.NoError(t, err)
	require.True(t, published)
	_, err = db.Operator.Core.NewUpdate().Model((*database.LfsMetaObject)(nil)).
		Set("xnet_dispatched_at = clock_timestamp() - INTERVAL '11 minutes'").
		Where("id = ?", first[0].ID).Exec(ctx)
	require.NoError(t, err)

	second, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Len(t, second, 1)
	published, err = fence.WithXnetDispatchPublishFence(ctx, first[0], func() error {
		publishCount++
		return nil
	})
	require.NoError(t, err)
	require.False(t, published)
	published, err = fence.WithXnetDispatchPublishFence(ctx, second[0], func() error {
		publishCount++
		return nil
	})
	require.NoError(t, err)
	require.True(t, published)

	require.NoError(t, pauseXnetDispatchForTest(ctx, db, 1))
	published, err = fence.WithXnetDispatchPublishFence(ctx, second[0], func() error {
		publishCount++
		return nil
	})
	require.NoError(t, err)
	require.False(t, published)
	require.Equal(t, 2, publishCount)
}

func TestLfsMetaStore_XnetDispatchPublishFencePreventsReclaimDuringPublish(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 1)
	fence := database.NewXnetDispatchFenceWithDB(db)
	oldClaimAt := time.Now().Add(-11 * time.Minute)

	_, err := store.Create(ctx, database.LfsMetaObject{RepositoryID: 1, Oid: "slow-publish"})
	require.NoError(t, err)
	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: oldClaimAt, LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	_, err = db.Operator.Core.NewUpdate().Model((*database.LfsMetaObject)(nil)).
		Set("xnet_dispatched_at = ?", oldClaimAt).Where("id = ?", claimed[0].ID).Exec(ctx)
	require.NoError(t, err)
	claimedObject, err := store.FindByOID(ctx, claimed[0].RepositoryID, claimed[0].Oid)
	require.NoError(t, err)
	claimed[0] = *claimedObject

	started := make(chan struct{})
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	type publishResult struct {
		published bool
		err       error
	}
	resultCh := make(chan publishResult, 1)
	go func() {
		published, publishErr := fence.WithXnetDispatchPublishFence(ctx, claimed[0], func() error {
			close(started)
			<-release
			return nil
		})
		resultCh <- publishResult{published: published, err: publishErr}
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for publish callback")
	}

	reclaimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Empty(t, reclaimed)
	close(release)
	var result publishResult
	select {
	case result = <-resultCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for fenced publish")
	}
	require.NoError(t, result.err)
	require.True(t, result.published)

	refreshed, err := store.FindByOID(ctx, claimed[0].RepositoryID, claimed[0].Oid)
	require.NoError(t, err)
	require.NotNil(t, refreshed.XnetDispatchedAt)
	require.True(t, refreshed.XnetDispatchedAt.After(oldClaimAt))
}

func TestLfsMetaStore_StaleDispatchFailureDoesNotOverwriteRenewedClaim(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	activateXnetIntentRepositories(t, db, 1)
	firstClaimAt := time.Now().Add(-11 * time.Minute)

	_, err := store.Create(ctx, database.LfsMetaObject{RepositoryID: 1, Oid: "stale-failure"})
	require.NoError(t, err)
	first, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		Now: firstClaimAt, LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Len(t, first, 1)
	_, err = db.Operator.Core.NewUpdate().Model((*database.LfsMetaObject)(nil)).
		Set("xnet_dispatched_at = ?", firstClaimAt).Where("id = ?", first[0].ID).Exec(ctx)
	require.NoError(t, err)
	firstClaim, err := store.FindByOID(ctx, first[0].RepositoryID, first[0].Oid)
	require.NoError(t, err)
	first[0] = *firstClaim
	second, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Len(t, second, 1)

	applied, err := store.FailXnetDispatch(ctx, first[0])
	require.NoError(t, err)
	require.False(t, applied)
	current, err := store.FindByOID(ctx, second[0].RepositoryID, second[0].Oid)
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchDispatched, current.XnetDispatchStatus)
	require.Equal(t, second[0].XnetDispatchAttempts, current.XnetDispatchAttempts)
	require.Equal(t, second[0].XnetDispatchedAt, current.XnetDispatchedAt)
}

func TestXnetPublishRateLimiter_UsesDatabaseClockAndDoesNotReserveFutureSlots(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	firstLimiter := database.NewXnetPublishRateLimiterWithDB(db)
	secondLimiter := database.NewXnetPublishRateLimiterWithDB(db)
	interval := 50 * time.Millisecond

	var wait time.Duration
	wait, acquired, err := firstLimiter.AcquireXnetPublishSlot(ctx, interval)
	require.NoError(t, err)
	require.True(t, acquired)
	require.Zero(t, wait)
	var firstHorizon time.Time
	require.NoError(t, db.Operator.Core.NewRaw(`
		SELECT next_allowed_at FROM xnet_dispatch_rate_limits WHERE name = 'publish'
	`).Scan(ctx, &firstHorizon))

	wait, acquired, err = secondLimiter.AcquireXnetPublishSlot(ctx, interval)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Positive(t, wait)
	require.LessOrEqual(t, wait, interval)

	secondWait, acquired, err := secondLimiter.AcquireXnetPublishSlot(ctx, interval)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Positive(t, secondWait)
	require.LessOrEqual(t, secondWait, wait)
	var unchangedHorizon time.Time
	require.NoError(t, db.Operator.Core.NewRaw(`
		SELECT next_allowed_at FROM xnet_dispatch_rate_limits WHERE name = 'publish'
	`).Scan(ctx, &unchangedHorizon))
	require.Equal(t, firstHorizon, unchangedHorizon)

	_, err = db.Operator.Core.NewRaw("SELECT pg_sleep(0.06)").Exec(ctx)
	require.NoError(t, err)
	wait, acquired, err = secondLimiter.AcquireXnetPublishSlot(ctx, interval)
	require.NoError(t, err)
	require.True(t, acquired)
	require.Zero(t, wait)

	wait, acquired, err = firstLimiter.AcquireXnetPublishSlot(ctx, interval)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Positive(t, wait)
	require.LessOrEqual(t, wait, interval)
}

func TestLfsMetaStore_StopFenceRollsBackPauseWhenCallbackFails(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	fence := database.NewXnetDispatchFenceWithDB(db)
	activateXnetIntentRepositories(t, db, 1)

	created, err := store.Create(ctx, database.LfsMetaObject{RepositoryID: 1, Oid: "stop-fenced"})
	require.NoError(t, err)
	stopErr := errors.New("purge failed")
	err = fence.WithXnetDispatchStopFence(ctx, &created.RepositoryID, func() error { return stopErr })
	require.ErrorIs(t, err, stopErr)

	paused, err := store.FindByOID(ctx, created.RepositoryID, created.Oid)
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPending, paused.XnetDispatchStatus)
	claimed, err := store.ClaimXnetDispatchObjects(ctx, database.XnetDispatchClaim{
		LeaseDuration: 10 * time.Minute, Limit: 1, MaxAttempts: 3,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
}

func TestLfsMetaStore_StopAllOnlyNormalizesDispatchedObjects(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	fence := database.NewXnetDispatchFenceWithDB(db)

	now := time.Now()
	objects := []database.LfsMetaObject{
		{RepositoryID: 1, Oid: "pending", XnetDispatchStatus: database.XnetDispatchPending},
		{RepositoryID: 1, Oid: "failed", XnetDispatchStatus: database.XnetDispatchFailed, XnetDispatchAttempts: 1},
		{RepositoryID: 1, Oid: "dispatched", XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &now, XnetDispatchAttempts: 1},
	}
	store := database.NewLfsMetaObjectStoreWithDB(db)
	for _, object := range objects {
		_, err := store.Create(ctx, object)
		require.NoError(t, err)
	}

	require.NoError(t, fence.WithXnetDispatchStopFence(ctx, nil, func() error { return nil }))

	pending, err := store.FindByOID(ctx, 1, "pending")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPending, pending.XnetDispatchStatus)
	failed, err := store.FindByOID(ctx, 1, "failed")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchFailed, failed.XnetDispatchStatus)
	dispatched, err := store.FindByOID(ctx, 1, "dispatched")
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchGlobalPause, dispatched.XnetDispatchStatus)
}

func TestLfsMetaStore_FailureDoesNotResumePausedDispatch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	dispatchedAt := time.Now()

	created, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID:         1,
		Oid:                  "paused-in-flight",
		XnetDispatchStatus:   database.XnetDispatchDispatched,
		XnetDispatchAttempts: 1,
		XnetDispatchedAt:     &dispatchedAt,
	})
	require.NoError(t, err)
	require.NoError(t, pauseXnetDispatchForTest(ctx, db, created.RepositoryID))

	applied, err := store.FailXnetDispatch(ctx, *created)
	require.NoError(t, err)
	require.False(t, applied)
	paused, err := store.FindByOID(ctx, created.RepositoryID, created.Oid)
	require.NoError(t, err)
	require.Equal(t, database.XnetDispatchPaused, paused.XnetDispatchStatus)
	require.Nil(t, paused.XnetNextAttemptAt)
}

// TestLfsMetaStoreBulkUpdateOrCreateUpdatesRepositorySize verifies LFS metadata and its repository total stay consistent.
func TestLfsMetaStoreBulkUpdateOrCreateUpdatesRepositorySize(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	repoStore := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	lfsStore := database.NewLfsMetaObjectStoreWithDB(db)

	repo, err := repoStore.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		Path: "ns/repo", GitPath: "models_ns/repo"}))
	require.NoError(t, err)

	err = lfsStore.BulkUpdateOrCreate(ctx, repo.ID, []database.LfsMetaObject{
		{RepositoryID: repo.ID, Oid: "one", Size: 10},
		{RepositoryID: repo.ID, Oid: "two", Size: 20},
	})
	require.NoError(t, err)
	repo, err = repoStore.FindById(ctx, repo.ID)
	require.NoError(t, err)
	require.Equal(t, int64(30), repo.LFSObjectsSize)

	err = lfsStore.BulkUpdateOrCreate(ctx, repo.ID, nil)
	require.NoError(t, err)
	repo, err = repoStore.FindById(ctx, repo.ID)
	require.NoError(t, err)
	require.Zero(t, repo.LFSObjectsSize)
}

func TestLfsMetaStore_UpdateXnetUsed(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewLfsMetaObjectStoreWithDB(db)

	// Create a test LFS object
	_, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID: 123,
		Oid:          "test-oid-123",
		Size:         1024,
		XnetUsed:     false,
	})
	require.Nil(t, err)

	// Verify initial state
	obj, err := store.FindByOID(ctx, 123, "test-oid-123")
	require.Nil(t, err)
	require.Equal(t, "test-oid-123", obj.Oid)
	require.Equal(t, false, obj.XnetUsed)

	// Update XnetUsed to true
	err = store.UpdateXnetUsed(ctx, 123, "test-oid-123", true)
	require.Nil(t, err)

	// Verify update
	obj, err = store.FindByOID(ctx, 123, "test-oid-123")
	require.Nil(t, err)
	require.Equal(t, true, obj.XnetUsed)

	// Update XnetUsed back to false
	err = store.UpdateXnetUsed(ctx, 123, "test-oid-123", false)
	require.Nil(t, err)

	// Verify update
	obj, err = store.FindByOID(ctx, 123, "test-oid-123")
	require.Nil(t, err)
	require.Equal(t, false, obj.XnetUsed)

	// Test updating non-existent object (should not error but affect 0 rows)
	err = store.UpdateXnetUsed(ctx, 999, "non-existent-oid", true)
	require.Nil(t, err)
}

func TestLfsMetaStore_BulkUpdateExistingByOIDs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewLfsMetaObjectStoreWithDB(db)

	// Seed rows with intentionally stale Existing values to verify that the
	// bulk update writes the checked object-storage state instead of trusting DB.
	_, err := store.Create(ctx, database.LfsMetaObject{
		RepositoryID: 123,
		Oid:          "existing-oid",
		Size:         1024,
		Existing:     false,
	})
	require.Nil(t, err)
	_, err = store.Create(ctx, database.LfsMetaObject{
		RepositoryID: 123,
		Oid:          "missing-oid",
		Size:         2048,
		Existing:     true,
	})
	require.Nil(t, err)
	_, err = store.Create(ctx, database.LfsMetaObject{
		RepositoryID: 456,
		Oid:          "missing-oid",
		Size:         4096,
		Existing:     true,
	})
	require.Nil(t, err)

	err = store.BulkUpdateExistingByOIDs(ctx, 123, []string{"existing-oid"}, []string{"missing-oid"})
	require.Nil(t, err)

	existingObj, err := store.FindByOID(ctx, 123, "existing-oid")
	require.Nil(t, err)
	require.True(t, existingObj.Existing)

	missingObj, err := store.FindByOID(ctx, 123, "missing-oid")
	require.Nil(t, err)
	require.False(t, missingObj.Existing)

	otherRepoObj, err := store.FindByOID(ctx, 456, "missing-oid")
	require.Nil(t, err)
	require.True(t, otherRepoObj.Existing)

	objs, err := store.FindByRepoID(ctx, 123)
	require.Nil(t, err)
	require.Len(t, objs, 2)
}

func TestLfsMetaStore_CheckIfAllMigratedToXnet(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewLfsMetaObjectStoreWithDB(db)

	// Test with no objects
	allMigrated, err := store.CheckIfAllMigratedToXnet(ctx, 123)
	require.Nil(t, err)
	require.True(t, allMigrated, "No objects should return true")

	// Create test objects
	testCases := []database.LfsMetaObject{
		{RepositoryID: 123, Oid: "oid-1", Size: 1024, XnetUsed: false},
		{RepositoryID: 123, Oid: "oid-2", Size: 2048, XnetUsed: false},
		{RepositoryID: 456, Oid: "oid-3", Size: 3072, XnetUsed: true}, // Different repo
	}

	for _, tc := range testCases {
		_, err := store.Create(ctx, tc)
		require.Nil(t, err)
	}

	// Test with some objects not migrated
	allMigrated, err = store.CheckIfAllMigratedToXnet(ctx, 123)
	require.Nil(t, err)
	require.False(t, allMigrated, "Should return false when some objects are not migrated")

	// Test with all objects migrated in another repo
	allMigrated, err = store.CheckIfAllMigratedToXnet(ctx, 456)
	require.Nil(t, err)
	require.True(t, allMigrated, "Should return true when all objects are migrated")

	// Update one object to migrated
	err = store.UpdateXnetUsed(ctx, 123, "oid-1", true)
	require.Nil(t, err)

	// Test with one migrated, one not migrated
	allMigrated, err = store.CheckIfAllMigratedToXnet(ctx, 123)
	require.Nil(t, err)
	require.False(t, allMigrated, "Should return false when not all objects are migrated")

	// Update all objects to migrated
	err = store.UpdateXnetUsed(ctx, 123, "oid-2", true)
	require.Nil(t, err)

	// Test with all objects migrated
	allMigrated, err = store.CheckIfAllMigratedToXnet(ctx, 123)
	require.Nil(t, err)
	require.True(t, allMigrated, "Should return true when all objects are migrated")
}

func TestLfsMetaStore_ExistsByOidExclRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewLfsMetaObjectStoreWithDB(db)

	oid := "test-oid-exists"
	repo1 := int64(1001)
	repo2 := int64(1002)

	// Test with no objects
	exists, err := store.ExistsByOidExclRepo(ctx, oid, repo1)
	require.Nil(t, err)
	require.False(t, exists)

	// Create object in repo1
	_, err = store.Create(ctx, database.LfsMetaObject{
		RepositoryID: repo1,
		Oid:          oid,
		Size:         1024,
	})
	require.Nil(t, err)

	// Should be false because we exclude repo1
	exists, err = store.ExistsByOidExclRepo(ctx, oid, repo1)
	require.Nil(t, err)
	require.False(t, exists)

	// Should be true because we exclude repo2, and repo1 has the oid
	exists, err = store.ExistsByOidExclRepo(ctx, oid, repo2)
	require.Nil(t, err)
	require.True(t, exists)

	// Create object in repo2 as well
	_, err = store.Create(ctx, database.LfsMetaObject{
		RepositoryID: repo2,
		Oid:          oid,
		Size:         1024,
	})
	require.Nil(t, err)

	// Now it should be true for both
	exists, err = store.ExistsByOidExclRepo(ctx, oid, repo1)
	require.Nil(t, err)
	require.True(t, exists)
}

func TestLfsMetaStore_ExistsByOidInActiveRepoExclRepoIgnoresSoftDeletedRepositories(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	repoStore := database.NewRepoStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	store := database.NewLfsMetaObjectStoreWithDB(db)

	repo1, err := repoStore.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		RepositoryType: types.ModelRepo, Path: "ns/deleting-one", Name: "deleting-one", GitPath: "models_ns/deleting-one",
	}))
	require.NoError(t, err)
	repo2, err := repoStore.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		RepositoryType: types.ModelRepo, Path: "ns/deleting-two", Name: "deleting-two", GitPath: "models_ns/deleting-two",
	}))
	require.NoError(t, err)
	activeRepo, err := repoStore.CreateRepo(ctx, withTupleNamespace(t, db, database.Repository{
		RepositoryType: types.ModelRepo, Path: "ns/active", Name: "active", GitPath: "models_ns/active",
	}))
	require.NoError(t, err)
	for _, repoID := range []int64{repo1.ID, repo2.ID, activeRepo.ID} {
		_, err = store.Create(ctx, database.LfsMetaObject{RepositoryID: repoID, Oid: "shared"})
		require.NoError(t, err)
	}

	_, err = db.Core.NewDelete().Model((*database.Repository)(nil)).Where("id = ? OR id = ?", repo1.ID, repo2.ID).Exec(ctx)
	require.NoError(t, err)
	exists, err := store.ExistsByOidInActiveRepoExclRepo(ctx, "shared", repo1.ID)
	require.NoError(t, err)
	require.True(t, exists)

	_, err = db.Core.NewDelete().Model(activeRepo).WherePK().Exec(ctx)
	require.NoError(t, err)
	exists, err = store.ExistsByOidInActiveRepoExclRepo(ctx, "shared", repo1.ID)
	require.NoError(t, err)
	require.False(t, exists, "another soft-deleted repository must not retain the shared object")
}

// The result-message path must ask existence questions, not repeatedly count a
// repository's growing history of completed objects.
type xnetStateQueryHook struct{ queries []string }

func (h *xnetStateQueryHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	h.queries = append(h.queries, event.Query)
	return ctx
}
func (*xnetStateQueryHook) AfterQuery(context.Context, *bun.QueryEvent) {}

func TestLfsMetaStore_XnetDispatchStateUsesExistenceQueries(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewLfsMetaObjectStoreWithDB(db)
	now := time.Now()
	expired := now.Add(-20 * time.Minute)
	future := now.Add(time.Hour)
	cases := []struct {
		name     string
		object   database.LfsMetaObject
		expected types.XnetDispatchState
		checks   int
	}{
		{"active final attempt", database.LfsMetaObject{XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &now, XnetDispatchAttempts: 3}, types.XnetDispatchState{Active: true}, 1},
		{"pending", database.LfsMetaObject{XnetDispatchStatus: database.XnetDispatchPending}, types.XnetDispatchState{Retryable: true}, 2},
		{"backoff still retryable", database.LfsMetaObject{XnetDispatchStatus: database.XnetDispatchFailed, XnetNextAttemptAt: &future, XnetDispatchAttempts: 1}, types.XnetDispatchState{Retryable: true}, 2},
		{"expired retryable", database.LfsMetaObject{XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &expired, XnetDispatchAttempts: 1}, types.XnetDispatchState{Retryable: true}, 2},
		{"failed exhausted", database.LfsMetaObject{XnetDispatchStatus: database.XnetDispatchFailed, XnetDispatchAttempts: 3}, types.XnetDispatchState{Exhausted: true}, 3},
		{"expired exhausted", database.LfsMetaObject{XnetDispatchStatus: database.XnetDispatchDispatched, XnetDispatchedAt: &expired, XnetDispatchAttempts: 3}, types.XnetDispatchState{Exhausted: true}, 3},
		{"paused", database.LfsMetaObject{XnetDispatchStatus: database.XnetDispatchPaused, XnetDispatchAttempts: 3}, types.XnetDispatchState{}, 3},
		{"global paused", database.LfsMetaObject{XnetDispatchStatus: database.XnetDispatchGlobalPause, XnetDispatchAttempts: 3}, types.XnetDispatchState{}, 3},
		{"completed legacy", database.LfsMetaObject{XnetUsed: true}, types.XnetDispatchState{}, 3},
	}
	hook := &xnetStateQueryHook{}
	db.BunDB.AddQueryHook(hook)
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			object := tc.object
			object.RepositoryID = int64(7000 + i)
			object.Oid = tc.name
			_, err := store.Create(ctx, object)
			require.NoError(t, err)
			hook.queries = nil
			state, err := store.GetXnetDispatchState(ctx, types.XnetDispatchStateQuery{RepositoryID: object.RepositoryID, LeaseDuration: 10 * time.Minute, MaxAttempts: 3})
			require.NoError(t, err)
			require.Equal(t, tc.expected, state)
			exists := 0
			for _, query := range hook.queries {
				require.NotContains(t, strings.ToLower(query), "count(")
				if strings.Contains(strings.ToLower(query), "exists") {
					exists++
				}
			}
			require.Equal(t, tc.checks, exists)
			hook.queries = nil
			migrated, err := store.CheckIfAllMigratedToXnet(ctx, object.RepositoryID)
			require.NoError(t, err)
			require.Equal(t, object.XnetUsed, migrated)
			for _, query := range hook.queries {
				require.NotContains(t, strings.ToLower(query), "count(")
			}
		})
	}
}

func pauseXnetDispatchForTest(ctx context.Context, db *database.DB, repoID int64) error {
	return database.NewXnetDispatchFenceWithDB(db).WithXnetDispatchStopFence(ctx, &repoID, func() error { return nil })
}
