package database_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestCustomerLicenseStoreOwnershipAndValidity(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	now := time.Now()
	owner := uuid.NewString()
	for _, row := range []struct {
		owner, product string
		start, end     time.Time
		deleted        bool
	}{
		{owner, "CSGLite", now.Add(-time.Hour), now.Add(time.Hour), false},
		{"other", "CSGLite", now.Add(-time.Hour), now.Add(2 * time.Hour), false},
		{owner, "CSGHub", now.Add(-time.Hour), now.Add(3 * time.Hour), false},
		{owner, "CSGLite", now.Add(-2 * time.Hour), now.Add(-time.Hour), false},
		{owner, "CSGLite", now.Add(time.Hour), now.Add(4 * time.Hour), false},
		{owner, "CSGLite", now.Add(-time.Hour), now.Add(5 * time.Hour), true},
	} {
		license := &database.License{Key: uuid.NewString(), Company: "customer", Product: row.product, Edition: "Enterprise", MaxUser: 1, UserUUID: row.owner, StartTime: row.start, ExpireTime: row.end}
		_, err := db.Core.NewInsert().Model(license).Exec(ctx)
		require.NoError(t, err)
		if row.deleted {
			require.NoError(t, database.NewLicenseStoreWithDB(db).Delete(ctx, *license))
		}
	}
	store := database.NewCustomerLicenseStoreWithDB(db)
	license, err := store.GetCurrentForUser(ctx, owner, "CSGLite")
	require.NoError(t, err)
	require.Equal(t, owner, license.UserUUID)
	require.WithinDuration(t, now.Add(time.Hour), license.ExpireTime, time.Millisecond)
	_, err = store.GetCurrentForUser(ctx, "missing", "CSGLite")
	require.ErrorIs(t, err, sql.ErrNoRows)
}
