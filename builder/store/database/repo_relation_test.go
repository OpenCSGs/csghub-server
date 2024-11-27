package database_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestRepoRelationStore_FromTo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepoRelationsStoreWithDB(db)
	relations := []*database.RepoRelation{
		{FromRepoID: 1, ToRepoID: 2},
		{FromRepoID: 1, ToRepoID: 3},
		{FromRepoID: 1, ToRepoID: 4},
		{FromRepoID: 3, ToRepoID: 5},
	}

	for _, rel := range relations {
		err := db.Core.NewInsert().Model(rel).Scan(ctx, rel)
		require.Nil(t, err)
	}

	rs, err := store.From(ctx, 1)
	require.Nil(t, err)
	ids := []int64{}
	for _, r := range rs {
		ids = append(ids, r.ToRepoID)
	}
	require.ElementsMatch(t, []int64{2, 3, 4}, ids)

	rs, err = store.To(ctx, 5)
	require.Nil(t, err)
	ids = []int64{}
	for _, r := range rs {
		ids = append(ids, r.FromRepoID)
	}
	require.ElementsMatch(t, []int64{3}, ids)

	err = store.Delete(ctx, 1, 3)
	require.Nil(t, err)
	rs, err = store.From(ctx, 1)
	require.Nil(t, err)
	ids = []int64{}
	for _, r := range rs {
		ids = append(ids, r.ToRepoID)
	}
	require.ElementsMatch(t, []int64{2, 4}, ids)

}

func TestRepoRelationStore_Override(t *testing.T) {
	cases := []struct {
		from        int64
		to          []int64
		expected1To []int64
		expected3To []int64
	}{
		{1, nil, []int64{}, []int64{5}},
		{1, []int64{2}, []int64{2}, []int64{5}},
		{1, []int64{5}, []int64{5}, []int64{5}},
		{1, []int64{2, 3}, []int64{2, 3}, []int64{5}},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			db := tests.InitTestDB()
			defer db.Close()
			ctx := context.TODO()

			store := database.NewRepoRelationsStoreWithDB(db)
			relations := []*database.RepoRelation{
				{FromRepoID: 1, ToRepoID: 2},
				{FromRepoID: 1, ToRepoID: 3},
				{FromRepoID: 1, ToRepoID: 4},
				{FromRepoID: 3, ToRepoID: 5},
			}

			for _, rel := range relations {
				err := db.Core.NewInsert().Model(rel).Scan(ctx, rel)
				require.Nil(t, err)
			}

			err := store.Override(ctx, c.from, c.to...)
			require.Nil(t, err)

			rs, err := store.From(ctx, 1)
			require.Nil(t, err)
			ids := []int64{}
			for _, r := range rs {
				ids = append(ids, r.ToRepoID)
			}
			require.ElementsMatch(t, c.expected1To, ids)

			rs, err = store.From(ctx, 3)
			require.Nil(t, err)
			ids = []int64{}
			for _, r := range rs {
				ids = append(ids, r.ToRepoID)
			}
			require.ElementsMatch(t, c.expected3To, ids)
		})
	}
}
