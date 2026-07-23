package component

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestTagComponent_CreateTag(t *testing.T) {
	ctx := context.TODO()
	req := types.CreateTag{
		Name:     "my first tag",
		Category: "testCategory",
		Group:    "testGroup",
		Scope:    "testScope",
		BuiltIn:  true,
	}

	newTag := database.Tag{
		Name:     req.Name,
		Category: req.Category,
		Group:    req.Group,
		Scope:    types.TagScope(req.Scope),
		BuiltIn:  req.BuiltIn,
	}

	t.Run("admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		tc.mocks.stores.TagMock().EXPECT().CreateTag(ctx, newTag).Return(&newTag, nil)
		tc.mocks.moderationClient.EXPECT().PassTextCheck(ctx, mock.Anything, req.Name).Return(&rpc.CheckResult{
			IsSensitive: false,
		}, nil)

		tag, err := tc.CreateTag(ctx, req)
		require.Nil(t, err)
		require.Equal(t, req.Name, tag.Name)
		require.Equal(t, true, tag.BuiltIn)
	})
}

func TestTagComponent_GetTagByID(t *testing.T) {
	ctx := context.TODO()
	t.Run("admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		tc.mocks.stores.TagMock().EXPECT().FindTagByID(ctx, int64(1)).Return(&database.Tag{ID: int64(1), Name: "test-tag"}, nil)

		tag, err := tc.GetTagByID(ctx, int64(1))
		require.Nil(t, err)
		require.Equal(t, int64(1), tag.ID)
		require.Equal(t, "test-tag", tag.Name)
	})
}

func TestTagComponent_UpdateTag(t *testing.T) {
	ctx := context.TODO()
	req := types.UpdateTag{
		Name:     "testTag",
		Category: "testCategory",
		Group:    "testGroup",
		Scope:    "testScope",
		BuiltIn:  true,
	}

	newTag := database.Tag{
		ID:       int64(1),
		Name:     req.Name,
		Category: req.Category,
		Group:    req.Group,
		Scope:    types.TagScope(req.Scope),
		BuiltIn:  req.BuiltIn,
	}

	t.Run("admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		tc.mocks.stores.TagMock().EXPECT().UpdateTagByID(ctx, &newTag).Return(&newTag, nil)
		tc.mocks.moderationClient.EXPECT().PassTextCheck(ctx, mock.Anything, req.Name).Return(&rpc.CheckResult{
			IsSensitive: false,
		}, nil)

		tag, err := tc.UpdateTag(ctx, int64(1), req)
		require.Nil(t, err)
		require.Equal(t, req.Name, tag.Name)
		require.Equal(t, true, tag.BuiltIn)
	})
}

func TestTagComponent_DeleteTag(t *testing.T) {
	ctx := context.TODO()
	t.Run("admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		tc.mocks.stores.TagMock().EXPECT().DeleteTagByID(ctx, int64(1)).Return(nil)

		err := tc.DeleteTag(ctx, int64(1))
		require.Nil(t, err)
	})
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

	data, err := tc.UpdateMetaTags(ctx, types.DatasetTagScope, "ns", "n", "")
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
		ctx, types.DatasetTagScope, "ns", "n", "pytorch_model_old.bin", "tf_model_new.h5",
	)
	require.Nil(t, err)

}

func TestTagComponent_UpdateRepoTagsByCategory(t *testing.T) {
	ctx := context.TODO()
	tc := initializeTestTagComponent(ctx, t)

	tc.mocks.stores.TagMock().EXPECT().AllCategories(ctx, types.DatasetTagScope).Return(
		[]database.TagCategory{
			{Name: "c", Scope: types.DatasetTagScope, AutoDetected: false},
		}, nil,
	)

	filter := &types.TagFilter{
		Categories: []string{"c"},
		Scopes:     []types.TagScope{types.DatasetTagScope},
	}
	tc.mocks.stores.TagMock().EXPECT().AllTags(ctx, filter).Return(
		[]*database.Tag{
			{Name: "t1", ID: 2},
		}, nil,
	)
	tc.mocks.stores.TagMock().EXPECT().ReplaceRepoTagsByCategoryAndSource(ctx, int64(1), "c", types.TagSourceManual, []int64{2}).Return(nil)

	err := tc.UpdateRepoTagsByCategory(ctx, types.DatasetTagScope, 1, "c", []string{"t1"})
	require.Nil(t, err)
}

func TestTagComponent_UpdateRepoTagsByCategory_AutoDetectedBlocked(t *testing.T) {
	ctx := context.TODO()
	tc := initializeTestTagComponent(ctx, t)

	tc.mocks.stores.TagMock().EXPECT().AllCategories(ctx, types.DatasetTagScope).Return(
		[]database.TagCategory{
			{Name: "framework", Scope: types.DatasetTagScope, AutoDetected: true},
		}, nil,
	)

	err := tc.UpdateRepoTagsByCategory(ctx, types.DatasetTagScope, 1, "framework", []string{"t1"})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "auto-detected and cannot be manually updated")
}

func TestTagComponent_AllCategories(t *testing.T) {
	ctx := context.TODO()

	t.Run("empty", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		tc.mocks.stores.TagMock().EXPECT().AllCategories(ctx, types.TagScope("")).Return([]database.TagCategory{}, nil)

		categories, err := tc.AllCategories(ctx)
		require.Nil(t, err)
		require.Equal(t, []types.RepoTagCategory{}, categories)
	})

	t.Run("preserve show_name", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		tc.mocks.stores.TagMock().EXPECT().AllCategories(ctx, types.TagScope("")).Return([]database.TagCategory{
			{ID: 1, Name: "test", ShowName: "测试中文名称", Scope: types.ModelTagScope, Enabled: true},
		}, nil)

		categories, err := tc.AllCategories(ctx)
		require.Nil(t, err)
		require.Equal(t, []types.RepoTagCategory{
			{ID: 1, Name: "test", ShowName: "测试中文名称", Scope: types.ModelTagScope, Enabled: true},
		}, categories)
	})
}

func TestTagComponent_CreateCategory(t *testing.T) {
	ctx := context.TODO()

	t.Run("admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		tc.mocks.stores.TagMock().EXPECT().CreateCategory(ctx, database.TagCategory{
			Name:  "test-cate",
			Scope: types.TagScope("test-scope"),
		}).Return(&database.TagCategory{
			ID:    1,
			Name:  "test-cate",
			Scope: "test-scope",
		}, nil)

		category, err := tc.CreateCategory(ctx, types.CreateCategory{
			Name:  "test-cate",
			Scope: "test-scope",
		})
		require.Nil(t, err)
		require.NotNil(t, category)
	})
}

func TestTagComponent_UpdateCategory(t *testing.T) {
	ctx := context.TODO()

	t.Run("admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		tc.mocks.stores.TagMock().EXPECT().UpdateCategory(ctx, database.TagCategory{
			ID:    int64(1),
			Name:  "test-cate",
			Scope: types.TagScope("test-scope"),
		}).Return(&database.TagCategory{
			ID:    1,
			Name:  "test-cate",
			Scope: "test-scope",
		}, nil)

		category, err := tc.UpdateCategory(ctx, types.UpdateCategory{
			Name:  "test-cate",
			Scope: "test-scope",
		}, int64(1))
		require.Nil(t, err)
		require.NotNil(t, category)
	})
}

func TestTagComponent_AllTagsWithPagination(t *testing.T) {
	ctx := context.TODO()

	t.Run("success with tags", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		filter := &types.TagFilter{
			Scopes:     []types.TagScope{types.ModelTagScope},
			Categories: []string{"task"},
		}
		dbTags := []*database.Tag{
			{ID: 1, Name: "tag1", Category: "task", Scope: types.ModelTagScope, BuiltIn: true, I18nKey: "tag1_key"},
			{ID: 2, Name: "tag2", Category: "task", Scope: types.ModelTagScope, BuiltIn: false, I18nKey: "tag2_key"},
		}
		tc.mocks.stores.TagMock().EXPECT().AllTagsWithPagination(ctx, filter, 10, 2).Return(dbTags, 15, nil)

		tags, total, err := tc.AllTagsWithPagination(ctx, filter, 10, 2)
		require.Nil(t, err)
		require.Equal(t, 15, total)
		require.Len(t, tags, 2)
		require.Equal(t, int64(1), tags[0].ID)
		require.Equal(t, "tag1", tags[0].Name)
		require.Equal(t, "task", tags[0].Category)
		require.Equal(t, types.ModelTagScope, tags[0].Scope)
		require.Equal(t, true, tags[0].BuiltIn)
		require.Equal(t, "tag1_key", tags[0].ShowName)
		require.Equal(t, "tag1_key", tags[0].I18nKey)
	})

	t.Run("empty result", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		filter := &types.TagFilter{Search: "nonexistent"}
		tc.mocks.stores.TagMock().EXPECT().AllTagsWithPagination(ctx, filter, 50, 1).Return([]*database.Tag{}, 0, nil)

		tags, total, err := tc.AllTagsWithPagination(ctx, filter, 50, 1)
		require.Nil(t, err)
		require.Equal(t, 0, total)
		require.Len(t, tags, 0)
	})

	t.Run("store error", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		filter := &types.TagFilter{}
		tc.mocks.stores.TagMock().EXPECT().AllTagsWithPagination(ctx, filter, 10, 1).Return(nil, 0, fmt.Errorf("db error"))

		tags, total, err := tc.AllTagsWithPagination(ctx, filter, 10, 1)
		require.NotNil(t, err)
		require.Equal(t, 0, total)
		require.Nil(t, tags)
	})
}

func TestTagComponent_DeleteCategory(t *testing.T) {
	ctx := context.TODO()

	t.Run("admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		tc.mocks.stores.TagMock().EXPECT().DeleteCategory(ctx, int64(1)).Return(nil)

		err := tc.DeleteCategory(ctx, int64(1))
		require.Nil(t, err)
	})
}
