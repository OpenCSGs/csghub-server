package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestTagComponent_AllTagsByScopeAndCategory(t *testing.T) {
	ctx := context.TODO()
	tc := initializeTestTagComponent(ctx, t)

	tc.mocks.stores.TagMock().EXPECT().AllTagsByScopeAndCategory(ctx, database.CodeTagScope, "cat").Return(
		[]*database.Tag{{Name: "t"}}, nil,
	)

	data, err := tc.AllTagsByScopeAndCategory(ctx, "code", "cat")
	require.Nil(t, err)
	require.Equal(t, []*database.Tag{{Name: "t"}}, data)
}

func TestTagComponent_ClearMetaTags(t *testing.T) {
	ctx := context.TODO()
	tc := initializeTestTagComponent(ctx, t)

	tc.mocks.stores.TagMock().EXPECT().SetMetaTags(
		ctx, types.ModelRepo, "ns", "n", []*database.Tag(nil),
	).Return(nil, nil)

	err := tc.ClearMetaTags(ctx, types.ModelRepo, "ns", "n")
	require.Nil(t, err)
}

func TestTagComponent_UpdateMetaTags(t *testing.T) {
	ctx := context.TODO()
	tc := initializeTestTagComponent(ctx, t)

	tc.mocks.stores.TagMock().EXPECT().AllDatasetTags(ctx).Return([]*database.Tag{}, nil)
	tc.mocks.stores.TagMock().EXPECT().SaveTags(ctx, []*database.Tag(nil)).Return(nil)
	tc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.DatasetRepo, "ns", "n").Return(
		&database.Repository{ID: 1}, nil,
	)
	tc.mocks.stores.TagMock().EXPECT().SetMetaTags(
		ctx, types.DatasetRepo, "ns", "n", []*database.Tag(nil),
	).Return(nil, nil)
	tc.mocks.stores.RepoMock().EXPECT().UpdateLicenseByTag(ctx, int64(1)).Return(nil)

	data, err := tc.UpdateMetaTags(ctx, database.DatasetTagScope, "ns", "n", "")
	require.Nil(t, err)
	require.Equal(t, []*database.RepositoryTag(nil), data)
}

func TestTagComponent_UpdateLibraryTags(t *testing.T) {
	ctx := context.TODO()
	tc := initializeTestTagComponent(ctx, t)

	tags := []*database.Tag{
		{Category: "framework", Name: "pytorch", ID: 1},
		{Category: "framework", Name: "tensorflow", ID: 2},
	}
	tc.mocks.stores.TagMock().EXPECT().AllDatasetTags(ctx).Return(tags, nil)
	tc.mocks.stores.TagMock().EXPECT().SetLibraryTag(
		ctx, types.DatasetRepo, "ns", "n", tags[1], tags[0],
	).Return(nil)

	err := tc.UpdateLibraryTags(
		ctx, database.DatasetTagScope, "ns", "n", "pytorch_model_old.bin", "tf_model_new.h5",
	)
	require.Nil(t, err)

}

func TestTagComponent_UpdateRepoTagsByCategory(t *testing.T) {
	ctx := context.TODO()
	tc := initializeTestTagComponent(ctx, t)

	tc.mocks.stores.TagMock().EXPECT().AllTagsByScopeAndCategory(ctx, database.DatasetTagScope, "c").Return(
		[]*database.Tag{
			{Name: "t1", ID: 2},
		}, nil,
	)
	tc.mocks.stores.RepoMock().EXPECT().TagIDs(ctx, int64(1), "c").Return([]int64{1}, nil)
	tc.mocks.stores.TagMock().EXPECT().UpsertRepoTags(ctx, int64(1), []int64{1}, []int64{2}).Return(nil)

	err := tc.UpdateRepoTagsByCategory(ctx, database.DatasetTagScope, 1, "c", []string{"t1"})
	require.Nil(t, err)
}
