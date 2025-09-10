package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestSSHKeyStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSSHKeyStoreWithDB(db)
	user := &database.User{
		Username: "user",
	}
	err := db.Core.NewInsert().Model(user).Scan(ctx, user)
	require.Nil(t, err)
	_, err = store.Create(ctx, &database.SSHKey{
		GitID:             123,
		FingerprintSHA256: "foo",
		UserID:            user.ID,
		Name:              "key",
		Content:           "content",
	})
	require.Nil(t, err)

	sh := &database.SSHKey{}
	err = db.Core.NewSelect().Model(sh).Where("git_id=?", 123).Scan(ctx)
	require.Nil(t, err)

	sh, err = store.FindByID(ctx, sh.ID)
	require.Nil(t, err)
	require.Equal(t, int64(123), sh.GitID)

	sh, err = store.FindByFingerpringSHA256(ctx, "foo")
	require.Nil(t, err)
	require.Equal(t, int64(123), sh.GitID)

	exist, err := store.IsExist(ctx, "user", "key")
	require.Nil(t, err)
	require.True(t, exist)

	shv, err := store.FindByUsernameAndName(ctx, "user", "key")
	require.Nil(t, err)
	require.Equal(t, int64(123), shv.GitID)

	sh, err = store.FindByKeyContent(ctx, "content")
	require.Nil(t, err)
	require.Equal(t, int64(123), sh.GitID)

	sh, err = store.FindByNameAndUserID(ctx, "key", user.ID)
	require.Nil(t, err)
	require.Equal(t, int64(123), sh.GitID)

	err = store.Delete(ctx, sh.ID)
	require.Nil(t, err)
	_, err = store.FindByID(ctx, sh.ID)
	require.NotNil(t, err)

}
