package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAccountAccessTokenRateStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountAccessTokenRateStoreWithDB(db)

	rate := &database.AccountAccessTokenRate{
		TokenID:    9001,
		AccessTime: 1760000000,
		Token:      120,
	}
	err := store.Create(ctx, rate)
	require.NoError(t, err)
	require.NotZero(t, rate.ID, "autoincrement id should be populated after insert")

	var got database.AccountAccessTokenRate
	err = db.Core.NewSelect().Model(&got).Where("id = ?", rate.ID).Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(9001), got.TokenID)
	require.Equal(t, int64(1760000000), got.AccessTime)
	require.Equal(t, int64(120), got.Token)

	// a second row must get its own id and not overwrite the first one
	other := &database.AccountAccessTokenRate{
		TokenID:    9001,
		AccessTime: 1760000060,
		Token:      30,
	}
	err = store.Create(ctx, other)
	require.NoError(t, err)
	require.NotEqual(t, rate.ID, other.ID)

	var count int
	err = db.Core.NewSelect().Model((*database.AccountAccessTokenRate)(nil)).
		ColumnExpr("COUNT(*)").
		Where("token_id = ?", int64(9001)).
		Scan(ctx, &count)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestAccountAccessTokenRateStore_GetStatByTokenID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountAccessTokenRateStoreWithDB(db)

	seed := []database.AccountAccessTokenRate{
		{TokenID: 9100, AccessTime: 1760000000, Token: 10},
		{TokenID: 9100, AccessTime: 1760000030, Token: 20},
		// row exactly at the window start is inclusive (access_time >= since)
		{TokenID: 9100, AccessTime: 1760000060, Token: 40},
		{TokenID: 9100, AccessTime: 1760000061, Token: 80},
		// zero-token access still counts as a request
		{TokenID: 9100, AccessTime: 1760000065},
		// before the window start: excluded
		{TokenID: 9100, AccessTime: 1759999999, Token: 999},
		// other token: excluded
		{TokenID: 9200, AccessTime: 1760000061, Token: 777},
	}
	for i := range seed {
		require.NoError(t, store.Create(ctx, &seed[i]))
	}

	stat, err := store.GetStatByTokenID(ctx, &database.AccountAccessTokenRate{
		TokenID:    9100,
		AccessTime: 1760000060,
	})
	require.NoError(t, err)
	require.Equal(t, 3, stat.Count)
	require.Equal(t, int64(120), stat.TokenSum)

	// a window after all rows of an existing token still yields a zeroed
	// stat, not an error (coalesce(sum(token), 0) covers the empty sum)
	stat, err = store.GetStatByTokenID(ctx, &database.AccountAccessTokenRate{
		TokenID:    9100,
		AccessTime: 2000000000,
	})
	require.NoError(t, err)
	require.Equal(t, 0, stat.Count)
	require.Equal(t, int64(0), stat.TokenSum)

	// an unknown token also yields a zeroed stat
	stat, err = store.GetStatByTokenID(ctx, &database.AccountAccessTokenRate{
		TokenID:    987654321,
		AccessTime: 0,
	})
	require.NoError(t, err)
	require.Equal(t, 0, stat.Count)
	require.Equal(t, int64(0), stat.TokenSum)
}

func TestAccountAccessTokenRateStore_DeleteOldByTokenID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountAccessTokenRateStoreWithDB(db)

	seed := []database.AccountAccessTokenRate{
		{TokenID: 9300, AccessTime: 1760000000, Token: 10},
		// exactly at the cutoff is deleted too (access_time <= cutoff)
		{TokenID: 9300, AccessTime: 1760000060, Token: 20},
		// newer than the cutoff: kept
		{TokenID: 9300, AccessTime: 1760000120, Token: 40},
		// another token is never touched by the delete
		{TokenID: 9400, AccessTime: 1760000000, Token: 999},
	}
	for i := range seed {
		require.NoError(t, store.Create(ctx, &seed[i]))
	}

	err := store.DeleteOldByTokenID(ctx, &database.AccountAccessTokenRate{
		TokenID:    9300,
		AccessTime: 1760000060,
	})
	require.NoError(t, err)

	remaining, err := store.GetStatByTokenID(ctx, &database.AccountAccessTokenRate{TokenID: 9300, AccessTime: 0})
	require.NoError(t, err)
	require.Equal(t, 1, remaining.Count)
	require.Equal(t, int64(40), remaining.TokenSum)

	other, err := store.GetStatByTokenID(ctx, &database.AccountAccessTokenRate{TokenID: 9400, AccessTime: 0})
	require.NoError(t, err)
	require.Equal(t, 1, other.Count)
	require.Equal(t, int64(999), other.TokenSum)

	// deleting an already-empty range is a no-op without error
	err = store.DeleteOldByTokenID(ctx, &database.AccountAccessTokenRate{
		TokenID:    9300,
		AccessTime: 1,
	})
	require.NoError(t, err)
}
