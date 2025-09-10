package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAccountUsersStore_AllMethods(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	au := database.NewAccountUserStoreWithDB(db)

	uid := uuid.New().String()
	err := au.Create(ctx, database.AccountUser{
		UserUUID:    uid,
		Balance:     100,
		CashBalance: 200,
	})
	require.Nil(t, err)

	dbu := &database.AccountUser{}
	err = db.Core.NewSelect().Model(dbu).Where("user_uuid=?", uid).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, float64(100), dbu.Balance)

	user, err := au.FindUserByID(ctx, uid)
	require.Nil(t, err)
	require.Equal(t, float64(100), user.Balance)
	require.Equal(t, float64(200), user.CashBalance)

	users, total, err := au.List(ctx, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, float64(100), users[0].Balance)
	require.Equal(t, float64(200), users[0].CashBalance)

	users, err = au.ListAllByUserUUID(ctx, uid)
	require.Nil(t, err)
	require.Equal(t, float64(100), users[0].Balance)
	require.Equal(t, float64(200), users[0].CashBalance)

	//SetLowBalanceWarn test
	err = au.SetLowBalanceWarn(ctx, uid, 500.0)
	require.Nil(t, err)

	// Verify low balance warn update
	updatedUser, err := au.FindUserByID(ctx, uid)
	require.Nil(t, err)
	require.Equal(t, float64(500.0), updatedUser.LowBalanceWarn)

	// SetLowBalanceWarnAtNow test
	err = au.SetLowBalanceWarnAtNow(ctx, uid)
	require.Nil(t, err)

	// Verify low_balance_warn_at is set and close to now
	updatedUser, err = au.FindUserByID(ctx, uid)
	require.Nil(t, err)

	// Optional: check that the timestamp is recent (within 2 seconds)
	require.WithinDuration(t, time.Now(), updatedUser.LowBalanceWarnAt, 2*time.Second)
}

func TestAccountUsersStore_UpdateNegativeBalanceWarnAt(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	au := database.NewAccountUserStoreWithDB(db)

	uid := uuid.New().String()
	err := au.Create(ctx, database.AccountUser{
		UserUUID:    uid,
		Balance:     100,
		CashBalance: 200,
	})
	require.Nil(t, err)

	err = au.UpdateNegativeBalanceWarnAt(ctx, uid, time.Now())
	require.Nil(t, err)

	user, err := au.FindUserByID(ctx, uid)
	require.Nil(t, err)
	require.Equal(t, false, user.NegativeBalanceWarnAt.IsZero())

	err = au.UpdateNegativeBalanceWarnAt(ctx, uid, time.Time{})
	require.Nil(t, err)

	user, err = au.FindUserByID(ctx, uid)
	require.Nil(t, err)
	require.Equal(t, true, user.NegativeBalanceWarnAt.IsZero())
}
