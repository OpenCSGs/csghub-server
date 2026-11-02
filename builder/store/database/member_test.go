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

func TestMemberStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	seedTupleOrganization(t, db, 123)
	defer db.Close()
	ctx := context.TODO()

	userStore := database.NewUserStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	user := &database.User{
		ID:       456,
		Username: "testuser",
		UUID:     uuid.New().String(),
		Password: "password",
	}
	err := userStore.Create(ctx, user, &database.Namespace{Path: "testuser"})
	require.Nil(t, err)

	store := database.NewMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))

	err = store.Add(ctx, 123, 456, string(types.UserRead))
	require.Nil(t, err)
	mem := &database.Member{}
	err = db.Core.NewSelect().Model(mem).Where("user_id=?", 456).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, string(types.UserRead), mem.Role)

	mem, err = store.Find(ctx, 123, 456)
	require.Nil(t, err)
	require.Equal(t, string(types.UserRead), mem.Role)

	ms, err := store.UserMembers(ctx, 456)
	require.Nil(t, err)
	require.Equal(t, 1, len(ms))
	require.Equal(t, string(types.UserRead), ms[0].Role)

	ms, count, err := store.OrganizationMembers(ctx, 123, "", 10, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(ms))
	require.Equal(t, 1, count)
	require.Equal(t, string(types.UserRead), ms[0].Role)
	require.Equal(t, user, ms[0].User)

	err = store.Update(ctx, 123, 456, string(types.UserWrite))
	require.Nil(t, err)

	err = store.Delete(ctx, 123, 456)
	require.Nil(t, err)
	_, err = store.Find(ctx, 123, 456)
	require.NotNil(t, err)

}

func TestMemberStore_OrgMembersWithNilUser(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	store := database.NewMemberStoreWithDB(db)

	// Simulate an orphaned historical row; normal writes require valid identities.
	_, err := db.Core.NewInsert().Model(&database.Member{OrganizationID: 123, UserID: 456, Role: string(types.UserRead)}).Exec(ctx)
	require.Nil(t, err)
	mem := &database.Member{}
	err = db.Core.NewSelect().Model(mem).Where("user_id=?", 456).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, string(types.UserRead), mem.Role)

	ms, err := store.UserMembers(ctx, 456)
	require.Nil(t, err)
	require.Equal(t, 1, len(ms))
	require.Equal(t, string(types.UserRead), ms[0].Role)

	ms, count, err := store.OrganizationMembers(ctx, 123, "", 10, 1)
	require.Nil(t, err)
	require.Equal(t, 0, len(ms))
	require.Equal(t, 0, count)
}

func TestMemberStore_UserUUIDsByOrganizationID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userStore := database.NewUserStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))
	memberStore := database.NewMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))

	orgID := int64(9999)
	seedTupleOrganization(t, db, orgID)
	role := string(types.UserRead)

	testUsers := []struct {
		GitID    int64
		Username string
	}{
		{GitID: 2001, Username: "u-001"},
		{GitID: 2002, Username: "u-002"},
		{GitID: 2003, Username: "u-003"},
	}

	var expectedUUIDs []string

	for _, u := range testUsers {
		userUUID := uuid.New().String()

		err := userStore.Create(ctx, &database.User{
			GitID:    u.GitID,
			Username: u.Username,
			UUID:     userUUID,
			Password: "secret",
		}, &database.Namespace{Path: u.Username})
		require.NoError(t, err)

		createdUser, err := userStore.FindByUsername(ctx, u.Username)
		require.NoError(t, err)

		err = memberStore.Add(ctx, orgID, createdUser.ID, role)
		require.NoError(t, err)

		expectedUUIDs = append(expectedUUIDs, userUUID)
	}

	resultUUIDs, err := memberStore.UserUUIDsByOrganizationID(ctx, orgID)
	require.NoError(t, err)
	require.ElementsMatch(t, expectedUUIDs, resultUUIDs)

	emptyUUIDs, err := memberStore.UserUUIDsByOrganizationID(ctx, 8888)
	require.NoError(t, err)
	require.Empty(t, emptyUUIDs)
}

func TestMemberStore_Update(t *testing.T) {
	db := tests.InitTestDB()
	seedTupleOrganization(t, db, 123)
	_, createErr := db.Core.NewInsert().Model(&database.User{ID: 456, Username: "member-update", UUID: uuid.NewString()}).Exec(context.Background())
	require.NoError(t, createErr)
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMemberStoreWithRebacTupleJobClient(db, tests.NewFixtureRebacTupleJobClient(db))

	err := store.Add(ctx, 123, 456, string(types.UserRead))
	require.Nil(t, err)

	err = store.Update(ctx, 123, 456, string(types.UserWrite))
	require.Nil(t, err)

	err = store.Update(ctx, 123, 1, string(types.UserAdmin))
	require.NotNil(t, err)

	mem, err := store.Find(ctx, 123, 456)
	require.Nil(t, err)
	require.Equal(t, string(types.UserWrite), mem.Role)
}
