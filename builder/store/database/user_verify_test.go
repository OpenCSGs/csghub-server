package database_test

import (
	"context"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
	"testing"
)

func TestUserVerifyStore(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	userStore := database.NewUserStoreWithDB(db)
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
		RealName:    "测试用户",
		Username:    user.Username,
		IDCardFront: "front_img_url",
		IDCardBack:  "back_img_url",
		Status:      "pending",
	}
	created, err := verifyStore.CreateUserVerify(ctx, userVerify)
	require.NoError(t, err)
	require.Equal(t, "测试用户", created.RealName)
	require.Equal(t, types.VerifyStatusPending, created.Status)

	updated, err := verifyStore.UpdateUserVerify(ctx, created.ID, types.VerifyStatusApproved, "已通过审核")
	require.NoError(t, err)
	require.Equal(t, types.VerifyStatusApproved, updated.Status)
	require.Equal(t, "已通过审核", updated.Reason)

	err = userStore.UpdateVerifyStatus(ctx, user.UUID, types.VerifyStatusApproved)
	require.NoError(t, err)

	freshUser, err := userStore.FindByUUID(ctx, user.UUID)
	require.NoError(t, err)
	require.Equal(t, types.VerifyStatusApproved, freshUser.VerifyStatus)
}
