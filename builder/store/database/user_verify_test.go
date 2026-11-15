package database_test

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// TestUserVerifyStore verifies manual review commits both statuses without a separate mirroring call.
func TestUserVerifyStore(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	userStore := database.NewUserStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	verifyStore := database.NewUserVerifyStoreWithDB(db)

	user := &database.User{
		GitID:    1001,
		NickName: "tester",
		Username: "verify_user",
		Email:    "verify_user@example.com",
		Password: "secret123",
		UUID:     "uuid-verify-user",
	}
	ns := &database.Namespace{
		Path: "verify_user",
	}
	err := userStore.Create(ctx, user, ns)
	require.NoError(t, err)

	userVerify := &database.UserVerify{
		UUID:        user.UUID,
		IDCardFront: "front_img_url",
		IDCardBack:  "back_img_url",
		Status:      "pending",
	}
	created, err := verifyStore.CreateUserVerify(ctx, userVerify)
	require.NoError(t, err)
	require.Equal(t, types.VerifyStatusPending, created.Status)

	updated, err := verifyStore.UpdateUserVerify(ctx, created.ID, types.VerifyStatusApproved, "已通过审核")
	require.NoError(t, err)
	require.Equal(t, types.VerifyStatusApproved, updated.Status)
	require.Equal(t, "已通过审核", updated.Reason)

	freshUser, err := userStore.FindByUUID(ctx, user.UUID)
	require.NoError(t, err)
	require.Equal(t, types.VerifyStatusApproved, freshUser.VerifyStatus)
}

// TestIdentityStoreLifecycle covers atomic status mirroring, restarts, stale updates and manual guards.
func TestIdentityStoreLifecycle(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	user := &database.User{UUID: "identity-store-user", Username: "identity-store-user", Email: "identity-store@example.com"}
	_, err := db.Core.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)
	store := database.NewIdentityStoreWithDB(db)
	proposed := &database.UserVerify{UUID: user.UUID, Provider: "alipay"}
	first, err := store.Begin(ctx, proposed)
	require.NoError(t, err)
	next := *proposed
	again, err := store.Begin(ctx, &next)
	require.NoError(t, err)
	require.Equal(t, first.ID, again.ID)
	require.True(t, again.UpdatedAt.After(first.UpdatedAt))
	require.ErrorIs(t, store.Attach(ctx, first, "stale-cert"), database.ErrIdentityConflict)
	first = again
	require.NoError(t, store.Attach(ctx, first, "cert-one"))
	first.VerificationID = "cert-one"
	require.NoError(t, store.Finish(ctx, first, types.VerifyStatusRejected, "expired"))
	second, err := store.Begin(ctx, &database.UserVerify{UUID: user.UUID, Provider: "alipay"})
	require.NoError(t, err)
	require.NoError(t, store.Attach(ctx, second, "cert-two"))
	second.VerificationID = "cert-two"
	require.ErrorIs(t, store.Finish(ctx, first, types.VerifyStatusApproved, ""), database.ErrIdentityConflict)
	require.ErrorIs(t, store.Attach(ctx, first, "cert-one"), database.ErrIdentityConflict)
	require.NoError(t, store.Finish(ctx, second, types.VerifyStatusApproved, ""))
	fresh := new(database.User)
	require.NoError(t, db.Core.NewSelect().Model(fresh).Where("uuid = ?", user.UUID).Scan(ctx))
	require.Equal(t, types.VerifyStatusApproved, fresh.VerifyStatus)
	_, err = store.Begin(ctx, proposed)
	require.ErrorIs(t, err, database.ErrIdentityConflict)
	manual := database.NewUserVerifyStoreWithDB(db)
	_, err = manual.UpdateUserVerify(ctx, second.ID, types.VerifyStatusRejected, "manual override")
	require.ErrorIs(t, err, database.ErrIdentityConflict)
	_, err = manual.CreateUserVerify(ctx, &database.UserVerify{UUID: user.UUID, IDCardFront: "front", IDCardBack: "back", Status: types.VerifyStatusPending})
	require.ErrorIs(t, err, database.ErrIdentityConflict)
	userStore := database.NewUserStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	require.NoError(t, userStore.UpdateVerifyStatus(ctx, user.UUID, types.VerifyStatusPending))
	require.NoError(t, db.Core.NewSelect().Model(fresh).Where("uuid = ?", user.UUID).Scan(ctx))
	require.Equal(t, types.VerifyStatusApproved, fresh.VerifyStatus)
	stored, err := store.Get(ctx, user.UUID)
	require.NoError(t, err)
	encoded, err := json.Marshal(stored)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "new-cipher")
	require.NotContains(t, string(encoded), "hmac-value")
	require.NotContains(t, string(encoded), "cert-two")
	require.NotContains(t, string(encoded), "测试用户")
	require.NotContains(t, string(encoded), "real_name")
}

// TestUserVerifyStoreExternalConflict preserves external records and user status for both guarded manual writes.
func TestUserVerifyStoreExternalConflict(t *testing.T) {
	for _, status := range []types.VerifyStatus{types.VerifyStatusPending, types.VerifyStatusRejected, types.VerifyStatusApproved} {
		t.Run(string(status), func(t *testing.T) {
			db := tests.InitTestDB()
			t.Cleanup(func() { _ = db.Close() })
			ctx := context.Background()
			user := &database.User{UUID: "manual-conflict", Username: "manual-conflict", Email: "manual-conflict@example.com", VerifyStatus: status}
			_, err := db.Core.NewInsert().Model(user).Exec(ctx)
			require.NoError(t, err)
			record := &database.UserVerify{UUID: user.UUID, Provider: "alipay", Status: status,
				VerificationID: "original-cert"}
			_, err = db.Core.NewInsert().Model(record).Exec(ctx)
			require.NoError(t, err)
			store := database.NewUserVerifyStoreWithDB(db)
			before, err := store.GetUserVerify(ctx, user.UUID)
			require.NoError(t, err)
			_, err = store.CreateUserVerify(ctx, &database.UserVerify{UUID: user.UUID,
				IDCardFront: "front.png", IDCardBack: "back.png", Status: types.VerifyStatusPending})
			require.ErrorIs(t, err, database.ErrIdentityConflict)
			reviewStatus := types.VerifyStatusApproved
			if status == types.VerifyStatusApproved {
				reviewStatus = types.VerifyStatusRejected
			}
			_, err = store.UpdateUserVerify(ctx, before.ID, reviewStatus, "manual override")
			require.ErrorIs(t, err, database.ErrIdentityConflict)
			after, err := store.GetUserVerify(ctx, user.UUID)
			require.NoError(t, err)
			require.Equal(t, before, after)
			var userStatus types.VerifyStatus
			require.NoError(t, db.Core.NewSelect().Table("users").Column("verify_status").Where("uuid = ?", user.UUID).Scan(ctx, &userStatus))
			require.Equal(t, status, userStatus)
			_, err = store.UpdateUserVerify(ctx, before.ID+1, types.VerifyStatusApproved, "")
			require.ErrorIs(t, err, database.ErrIdentityConflict, "missing review IDs share the zero-row conflict contract")
		})
	}
}

// TestUserVerifyStoreUpdateErrors distinguishes zero-row conflicts from execution and row-count failures.
func TestUserVerifyStoreUpdateErrors(t *testing.T) {
	backendErr := errors.New("database result unavailable")
	for _, tc := range []struct {
		name     string
		result   driver.Result
		execErr  error
		wantErr  error
		wantText string
	}{
		{name: "zero rows", result: sqlmock.NewResult(0, 0), wantErr: database.ErrIdentityConflict},
		{name: "execution failure", execErr: backendErr, wantErr: backendErr},
		{name: "row count failure", result: sqlmock.NewErrorResult(backendErr), wantErr: backendErr},
		{name: "unexpected row count", result: sqlmock.NewResult(0, 2), wantText: "affected 2 row(s), want 1"},
		{name: "unknown row count", result: sqlmock.NewResult(0, -1), wantText: "affected -1 row(s), want 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sqlDB, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			core := bun.NewDB(sqlDB, pgdialect.New())
			t.Cleanup(func() { _ = core.Close() })
			sqlMock.ExpectQuery(`SELECT .* FROM "user_verifies"`).WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "provider", "status", "updated_at"}).AddRow(42, "user", "manual", "pending", time.Now()))
			sqlMock.ExpectBegin()
			sqlMock.ExpectQuery(`SELECT "id" FROM "users".*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			expected := sqlMock.ExpectExec(`UPDATE "user_verifies"`)
			if tc.execErr != nil {
				expected.WillReturnError(tc.execErr)
			} else {
				expected.WillReturnResult(tc.result)
			}
			sqlMock.ExpectRollback()
			store := database.NewUserVerifyStoreWithDB(&database.DB{Operator: database.Operator{Core: core}, BunDB: core})
			_, err = store.UpdateUserVerify(context.Background(), 42, types.VerifyStatusApproved, "")
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.EqualError(t, err, tc.wantText)
			}
			if !errors.Is(tc.wantErr, database.ErrIdentityConflict) {
				require.NotErrorIs(t, err, database.ErrIdentityConflict)
			}
			require.NoError(t, sqlMock.ExpectationsWereMet())
		})
	}
}

// TestUserVerifyStoreAtomicRollback ensures failed user-status mirroring rolls back the review.
func TestUserVerifyStoreAtomicRollback(t *testing.T) {
	sqlDB, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	core := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() { _ = core.Close() })
	backendErr := errors.New("user status update failed")
	sqlMock.ExpectQuery(`SELECT .* FROM "user_verifies"`).WillReturnRows(sqlmock.NewRows([]string{"id", "uuid", "provider", "status", "updated_at"}).AddRow(42, "user", "manual", "pending", time.Now()))
	sqlMock.ExpectBegin()
	sqlMock.ExpectQuery(`SELECT "id" FROM "users".*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	sqlMock.ExpectExec(`UPDATE "user_verifies"`).WillReturnResult(sqlmock.NewResult(0, 1))
	sqlMock.ExpectExec(`UPDATE "users"`).WillReturnError(backendErr)
	sqlMock.ExpectRollback()
	store := database.NewUserVerifyStoreWithDB(&database.DB{Operator: database.Operator{Core: core}, BunDB: core})
	result, err := store.UpdateUserVerify(context.Background(), 42, types.VerifyStatusApproved, "")
	require.ErrorIs(t, err, backendErr)
	require.Nil(t, result)
	require.NoError(t, sqlMock.ExpectationsWereMet())
}

// TestVerificationCompletionConcurrency uses real PostgreSQL sessions to prove only one request can complete each record.
func TestVerificationCompletionConcurrency(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	for _, provider := range []string{"manual", "alipay"} {
		t.Run(provider, func(t *testing.T) {
			user := &database.User{UUID: "completion-" + provider, Username: "completion-" + provider, Email: provider + "@completion.test", VerifyStatus: types.VerifyStatusPending}
			_, err := db.Core.NewInsert().Model(user).Exec(ctx)
			require.NoError(t, err)
			record := &database.UserVerify{UUID: user.UUID, Provider: provider, Status: types.VerifyStatusPending}
			_, err = db.Core.NewInsert().Model(record).Exec(ctx)
			require.NoError(t, err)
			// Reload PostgreSQL's timestamp precision before using the generation guard.
			manual := database.NewUserVerifyStoreWithDB(db)
			record, err = manual.GetUserVerify(ctx, user.UUID)
			require.NoError(t, err)
			original := *record
			start, results := make(chan struct{}), make(chan error, 2)
			var wg sync.WaitGroup
			for range 2 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					if provider == "manual" {
						_, err := manual.UpdateUserVerify(ctx, record.ID, types.VerifyStatusApproved, "")
						results <- err
					} else {
						results <- database.NewIdentityStoreWithDB(db).Finish(ctx, record, types.VerifyStatusApproved, "")
					}
				}()
			}
			close(start)
			wg.Wait()
			close(results)
			var succeeded, conflicted int
			for err := range results {
				if err == nil {
					succeeded++
				} else {
					require.ErrorIs(t, err, database.ErrIdentityConflict)
					conflicted++
				}
			}
			require.Equal(t, 1, succeeded)
			require.Equal(t, 1, conflicted)
			require.Equal(t, original, *record, "completion must not mutate a shared snapshot")
			fresh, err := manual.GetUserVerify(ctx, user.UUID)
			require.NoError(t, err)
			require.Equal(t, types.VerifyStatusApproved, fresh.Status)
			var status types.VerifyStatus
			require.NoError(t, db.Core.NewSelect().Table("users").Column("verify_status").Where("uuid = ?", user.UUID).Scan(ctx, &status))
			require.Equal(t, types.VerifyStatusApproved, status)
		})
	}
}

// TestManualVerificationGeneration rejects a decision computed before replacement material was submitted.
func TestManualVerificationGeneration(t *testing.T) {
	db := tests.InitTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	user := &database.User{UUID: "manual-generation", Username: "manual-generation", Email: "generation@example.com"}
	_, err := db.Core.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)
	store := database.NewUserVerifyStoreWithDB(db)
	_, err = store.CreateUserVerify(ctx, &database.UserVerify{UUID: user.UUID, Status: types.VerifyStatusPending})
	require.NoError(t, err)
	old, err := store.GetUserVerify(ctx, user.UUID)
	require.NoError(t, err)
	// Replacement uses the current submission time even if the stored timestamp is in the future.
	old.UpdatedAt = time.Now().Add(time.Minute).Truncate(time.Microsecond)
	_, err = db.Core.NewUpdate().Table("user_verifies").Set("updated_at = ?", old.UpdatedAt).Where("id = ?", old.ID).Exec(ctx)
	require.NoError(t, err)
	submittedAt := time.Now().Truncate(time.Microsecond)
	replacement, err := store.CreateUserVerify(ctx, &database.UserVerify{UUID: user.UUID, Status: types.VerifyStatusPending})
	require.NoError(t, err)
	fresh, err := store.GetUserVerify(ctx, user.UUID)
	require.NoError(t, err)
	require.False(t, fresh.UpdatedAt.Before(submittedAt))
	require.False(t, fresh.UpdatedAt.After(time.Now()))
	require.True(t, fresh.UpdatedAt.Before(old.UpdatedAt))
	require.True(t, fresh.UpdatedAt.Equal(replacement.UpdatedAt))
	require.ErrorIs(t, database.NewIdentityStoreWithDB(db).Finish(ctx, old, types.VerifyStatusApproved, ""), database.ErrIdentityConflict)
}

// TestIdentityStoreRestart replaces unfinished data while rejecting stale writes and approved identities.
func TestIdentityStoreRestart(t *testing.T) {
	for _, tc := range []struct {
		name       string
		provider   string
		status     types.VerifyStatus
		userStatus types.VerifyStatus
		certifyID  string
		conflict   bool
	}{
		{name: "pending without provider ID", provider: "alipay", status: types.VerifyStatusPending},
		{name: "pending with provider ID", provider: "alipay", status: types.VerifyStatusPending, certifyID: "old-cert"},
		{name: "rejected", provider: "alipay", status: types.VerifyStatusRejected, userStatus: types.VerifyStatusRejected, certifyID: "old-cert"},
		{name: "manual pending", provider: "manual", status: types.VerifyStatusPending},
		{name: "approved user", provider: "alipay", status: types.VerifyStatusPending, userStatus: types.VerifyStatusApproved, conflict: true},
		{name: "approved record", provider: "alipay", status: types.VerifyStatusApproved, conflict: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := tests.InitTestDB()
			t.Cleanup(func() { _ = db.Close() })
			ctx := context.Background()
			user := &database.User{UUID: "identity-restart", Username: "identity-restart", Email: "identity-restart@example.com", VerifyStatus: tc.userStatus}
			_, err := db.Core.NewInsert().Model(user).Exec(ctx)
			require.NoError(t, err)
			old := &database.UserVerify{UUID: user.UUID, Provider: tc.provider, Status: tc.status,
				VerificationID: tc.certifyID,
				IDCardFront:    "front", IDCardBack: "back", Reason: "old-reason"}
			// A future timestamp also verifies monotonic generation changes after clock adjustments.
			old.UpdatedAt = time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond)
			_, err = db.Core.NewInsert().Model(old).Returning("*").Exec(ctx)
			require.NoError(t, err)
			store := database.NewIdentityStoreWithDB(db)
			next, err := store.Begin(ctx, &database.UserVerify{UUID: user.UUID, Provider: "alipay"})
			if tc.conflict {
				require.ErrorIs(t, err, database.ErrIdentityConflict)
				stored, err := store.Get(ctx, user.UUID)
				require.NoError(t, err)
				require.Equal(t, old.Status, stored.Status)
				require.Equal(t, old.Provider, stored.Provider)
				return
			}
			require.NoError(t, err)
			require.Equal(t, old.ID, next.ID)
			require.True(t, next.UpdatedAt.After(old.UpdatedAt))
			require.Empty(t, next.VerificationID)
			require.ErrorIs(t, store.Attach(ctx, old, "late-cert"), database.ErrIdentityConflict)
			for _, status := range []types.VerifyStatus{types.VerifyStatusApproved, types.VerifyStatusRejected} {
				require.ErrorIs(t, store.Finish(ctx, old, status, "late-result"), database.ErrIdentityConflict)
			}
			stored, err := store.Get(ctx, user.UUID)
			require.NoError(t, err)
			require.Equal(t, "alipay", stored.Provider)
			require.Equal(t, types.VerifyStatusPending, stored.Status)
			require.Empty(t, stored.VerificationID)
			require.Empty(t, stored.Reason)
			require.Empty(t, stored.IDCardFront)
			require.Empty(t, stored.IDCardBack)
			fresh := new(database.User)
			require.NoError(t, db.Core.NewSelect().Model(fresh).Where("uuid = ?", user.UUID).Scan(ctx))
			require.Equal(t, types.VerifyStatusPending, fresh.VerifyStatus)
		})
	}
}

// TestIdentityJSONOmitsPersonalData checks both providers expose no removed fields.
func TestIdentityJSONOmitsPersonalData(t *testing.T) {
	for _, provider := range []string{"alipay", "manual"} {
		encoded, err := json.Marshal(database.UserVerify{Provider: provider, VerificationID: "private-cert", IDNumberHash: "internal-hash-token"})
		require.NoError(t, err)
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(encoded, &decoded))
		for _, field := range []string{"real_name", "id_number", "id_number_encrypted", "id_number_hash", "verification_id"} {
			require.NotContains(t, decoded, field)
		}
		require.NotContains(t, string(encoded), "private-cert")
		require.NotContains(t, string(encoded), "internal-hash-token")
	}
}

// TestUserVerifySchemaRetainsLegacyColumns preserves legacy columns and the non-unique hash index.
func TestUserVerifySchemaRetainsLegacyColumns(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	var count int
	err := db.Core.NewRaw(`SELECT count(*) FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'user_verifies' AND column_name IN ('real_name', 'id_number_encrypted')`).Scan(context.Background(), &count)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	err = db.Core.NewRaw(`SELECT count(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname = 'idx_user_verifies_id_number_hash'`).Scan(context.Background(), &count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	var definition string
	require.NoError(t, db.Core.NewRaw(`SELECT indexdef FROM pg_indexes WHERE schemaname = 'public' AND indexname = 'idx_user_verifies_id_number_hash'`).Scan(context.Background(), &definition))
	require.NotContains(t, definition, "UNIQUE")
}

// TestUserVerifyWritesWithoutIdentityText exercises both writers without accessing legacy identity columns.
func TestUserVerifyWritesWithoutIdentityText(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewIdentityStoreWithDB(db)
	for _, provider := range []string{"manual", "alipay"} {
		uuid := "new-identity-" + provider
		var err error
		if provider == "manual" {
			_, err = database.NewUserVerifyStoreWithDB(db).CreateUserVerify(ctx, &database.UserVerify{
				UUID: uuid, Username: uuid, IDCardFront: "front", IDCardBack: "back", Status: types.VerifyStatusPending,
			})
		} else {
			_, err = db.Core.NewInsert().Model(&database.User{UUID: uuid, Username: uuid, Email: uuid + "@example.com"}).Exec(ctx)
			require.NoError(t, err)
			_, err = store.Begin(ctx, &database.UserVerify{UUID: uuid, Provider: provider, IDNumberHash: "new-keyed-hash"})
		}
		require.NoError(t, err)
		record, err := store.Get(ctx, uuid)
		require.NoError(t, err)
		require.NotNil(t, record)
		require.Equal(t, provider, record.Provider)
		require.Equal(t, types.VerifyStatusPending, record.Status)
	}
}

// TestIdentityRewardConcurrency proves different accounts can verify concurrently but only one owns rewards.
func TestIdentityRewardConcurrency(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewIdentityStoreWithDB(db)
	var records []*database.UserVerify
	for _, uuid := range []string{"first-identity-user", "second-identity-user"} {
		_, err := db.Core.NewInsert().Model(&database.User{UUID: uuid, Username: uuid, Email: uuid + "@example.com"}).Exec(ctx)
		require.NoError(t, err)
		record, err := store.Begin(ctx, &database.UserVerify{UUID: uuid, Provider: "alipay", IDNumberHash: "same-identity-hash"})
		require.NoError(t, err)
		records = append(records, record)
	}
	start := make(chan struct{})
	results := make(chan error, len(records))
	for _, record := range records {
		go func() { <-start; results <- store.Finish(ctx, record, types.VerifyStatusApproved, "") }()
	}
	close(start)
	for range records {
		require.NoError(t, <-results)
	}
	owners := 0
	for _, record := range records {
		fresh, err := store.Get(ctx, record.UUID)
		require.NoError(t, err)
		require.Equal(t, types.VerifyStatusApproved, fresh.Status)
		eligible, err := store.IsFirstIdentityVerification(ctx, record.UUID)
		require.NoError(t, err)
		if eligible {
			owners++
		}
		again, err := store.IsFirstIdentityVerification(ctx, record.UUID)
		require.NoError(t, err)
		require.Equal(t, eligible, again)
	}
	require.Equal(t, 1, owners)

}

// TestIdentityRewardStates permits rewards only for successful, identifiable verification.
func TestIdentityRewardStates(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewIdentityStoreWithDB(db)
	for _, tc := range []struct {
		uuid, hash string
		status     types.VerifyStatus
		eligible   bool
	}{
		{"failed", "reusable-hash", types.VerifyStatusRejected, false},
		{"successful", "reusable-hash", types.VerifyStatusApproved, true},
		{"missing-hash", "", types.VerifyStatusApproved, false},
	} {
		_, err := db.Core.NewInsert().Model(&database.User{UUID: tc.uuid, Username: tc.uuid, Email: tc.uuid + "@example.com"}).Exec(ctx)
		require.NoError(t, err)
		record, err := store.Begin(ctx, &database.UserVerify{UUID: tc.uuid, Provider: "alipay", IDNumberHash: tc.hash})
		require.NoError(t, err)
		eligible, err := store.IsFirstIdentityVerification(ctx, tc.uuid)
		require.NoError(t, err)
		require.False(t, eligible)
		require.NoError(t, store.Finish(ctx, record, tc.status, ""))
		eligible, err = store.IsFirstIdentityVerification(ctx, tc.uuid)
		require.NoError(t, err)
		require.Equal(t, tc.eligible, eligible)
	}
}

// TestIdentityRewardHistoricalRecords uses existing approved records and ignores unsuccessful attempts.
func TestIdentityRewardHistoricalRecords(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	_, err := db.Core.ExecContext(ctx, `INSERT INTO user_verifies (uuid, username, provider, status, id_number_hash, updated_at) VALUES
 ('old-first', 'first', 'alipay', 'approved', 'old-hash', '2026-01-01'),
 ('old-second', 'second', 'alipay', 'approved', 'old-hash', '2026-01-02'),
 ('old-failed', 'failed', 'alipay', 'rejected', 'failed-hash', '2026-01-01'),
 ('old-manual', 'manual', 'manual', 'approved', NULL, '2026-01-01')`)
	require.NoError(t, err)
	store := database.NewIdentityStoreWithDB(db)
	for uuid, expected := range map[string]bool{"old-first": true, "old-second": false, "old-failed": false, "old-manual": false} {
		eligible, err := store.IsFirstIdentityVerification(ctx, uuid)
		require.NoError(t, err)
		require.Equal(t, expected, eligible, uuid)
	}
}

// TestIdentityRewardCompletionOrder uses approval order, even when the older submission finishes later.
func TestIdentityRewardCompletionOrder(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewIdentityStoreWithDB(db)
	var records []*database.UserVerify
	for _, uuid := range []string{"submitted-first", "approved-first"} {
		_, err := db.Core.NewInsert().Model(&database.User{UUID: uuid, Username: uuid, Email: uuid + "@example.com"}).Exec(ctx)
		require.NoError(t, err)
		record, err := store.Begin(ctx, &database.UserVerify{UUID: uuid, Provider: "alipay", IDNumberHash: "completion-order-hash"})
		require.NoError(t, err)
		records = append(records, record)
	}
	require.NoError(t, store.Finish(ctx, records[1], types.VerifyStatusApproved, ""))
	// Simulate an earlier approval from a replica with a clock ahead of this process.
	future := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	_, err := db.Core.NewUpdate().Table("user_verifies").Set("updated_at = ?", future).Where("id = ?", records[1].ID).Exec(ctx)
	require.NoError(t, err)
	eligible, err := store.IsFirstIdentityVerification(ctx, records[1].UUID)
	require.NoError(t, err)
	require.True(t, eligible)
	require.NoError(t, store.Finish(ctx, records[0], types.VerifyStatusApproved, ""))
	for i, record := range records {
		eligible, err := store.IsFirstIdentityVerification(ctx, record.UUID)
		require.NoError(t, err)
		require.Equal(t, i == 1, eligible)
	}
}
