//go:build saas || ee

package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	database "opencsg.com/csghub-server/builder/store/database/ee"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestModelTreeStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewModelTreeWithDB(db)
	err := store.Add(ctx, types.ModelTreeReq{
		SourceRepoID: 123,
		SourcePath:   "foo/bar",
		TargetRepoID: 111,
		TargetPath:   "foo/bar2",
	})
	require.Nil(t, err)

	m := &database.ModelTree{}
	err = db.Core.NewSelect().Model(m).Where("target_repo_id=?", 111).Scan(ctx)
	require.Nil(t, err)
	// update
	err = store.Add(ctx, types.ModelTreeReq{
		SourceRepoID: 123,
		SourcePath:   "foo/bar1",
		TargetRepoID: 111,
		TargetPath:   "foo/bar2",
		Relation:     types.RelationFinetune,
	})
	require.Nil(t, err)
	m = &database.ModelTree{}
	err = db.Core.NewSelect().Model(m).Where("target_repo_id=?", 111).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foo/bar1", m.SourcePath)
	//delete
	err = store.Delete(ctx, 111)
	require.Nil(t, err)
}

func TestModelTreeStore_GetParent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewModelTreeWithDB(db)
	err := store.Add(ctx, types.ModelTreeReq{
		SourceRepoID: 110,
		SourcePath:   "foo/bar",
		TargetRepoID: 111,
		TargetPath:   "foo/bar2",
		Relation:     types.RelationFinetune,
	})
	require.Nil(t, err)
	err = store.Add(ctx, types.ModelTreeReq{
		SourceRepoID: 111,
		SourcePath:   "foo/bar",
		TargetRepoID: 112,
		TargetPath:   "foo/bar2",
		Relation:     types.RelationFinetune,
	})
	require.Nil(t, err)
	nodes, err := store.GetParent(ctx, 112)
	require.Nil(t, err)
	require.Equal(t, 2, len(nodes))
	require.Equal(t, "foo/bar", nodes[1].SourcePath)
}

func TestModelTreeStore_GetRelationCount(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewModelTreeWithDB(db)
	err := store.Add(ctx, types.ModelTreeReq{
		SourceRepoID: 110,
		SourcePath:   "foo/bar",
		TargetRepoID: 111,
		TargetPath:   "foo/bar1",
		Relation:     types.RelationFinetune,
	})
	require.Nil(t, err)
	err = store.Add(ctx, types.ModelTreeReq{
		SourceRepoID: 111,
		SourcePath:   "foo/bar",
		TargetRepoID: 112,
		TargetPath:   "foo/bar2",
		Relation:     types.RelationFinetune,
	})
	require.Nil(t, err)
	err = store.Add(ctx, types.ModelTreeReq{
		SourceRepoID: 111,
		SourcePath:   "foo/bar",
		TargetRepoID: 113,
		TargetPath:   "foo/bar3",
		Relation:     types.RelationFinetune,
	})
	require.Nil(t, err)
	err = store.Add(ctx, types.ModelTreeReq{
		SourceRepoID: 111,
		SourcePath:   "foo/bar",
		TargetRepoID: 114,
		TargetPath:   "foo/bar4",
		Relation:     types.RelationAdapter,
	})
	require.Nil(t, err)
	err = store.Add(ctx, types.ModelTreeReq{
		SourceRepoID: 111,
		SourcePath:   "foo/bar",
		TargetRepoID: 115,
		TargetPath:   "foo/bar5",
		Relation:     types.RelationQuantized,
	})
	require.Nil(t, err)
	num, err := store.GetSourceRelationCount(ctx, 111, types.RelationFinetune)
	require.Nil(t, err)
	require.Equal(t, 2, num)
	num, err = store.GetSourceRelationCount(ctx, 111, types.RelationAdapter)
	require.Nil(t, err)
	require.Equal(t, 1, num)
	num, err = store.GetSourceRelationCount(ctx, 111, types.RelationQuantized)
	require.Nil(t, err)
	require.Equal(t, 1, num)
}
