package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestSkillVersionStore_LatestBySkillIDs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSkillVersionStoreWithDB(db)

	_, err := store.Create(ctx, database.SkillVersion{SkillID: 1, Version: "v1.0.0"})
	require.NoError(t, err)
	latestSkillOne, err := store.Create(ctx, database.SkillVersion{SkillID: 1, Version: "v1.1.0"})
	require.NoError(t, err)
	latestSkillTwo, err := store.Create(ctx, database.SkillVersion{SkillID: 2, Version: "v2.0.0"})
	require.NoError(t, err)

	versions, err := store.LatestBySkillIDs(ctx, []int64{1, 2, 3})

	require.NoError(t, err)
	require.Len(t, versions, 2)
	require.Equal(t, latestSkillOne.ID, versions[1].ID)
	require.Equal(t, latestSkillOne.Version, versions[1].Version)
	require.Equal(t, latestSkillTwo.ID, versions[2].ID)
	require.Equal(t, latestSkillTwo.Version, versions[2].Version)
}
