package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestTagComponent_CreateTag(t *testing.T) {
	ctx := context.TODO()

	username := "testUser"

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

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, username).Return(database.User{UUID: "testUUID", RoleMask: "admin"}, nil)
		tc.mocks.stores.TagMock().EXPECT().FindOrCreate(ctx, newTag).Return(&newTag, nil)
		tc.mocks.moderationClient.EXPECT().PassTextCheck(ctx, mock.Anything, req.Name).Return(&rpc.CheckResult{
			IsSensitive: false,
		}, nil)

		tag, err := tc.CreateTag(ctx, username, req)
		require.Nil(t, err)
		require.Equal(t, req.Name, tag.Name)
		require.Equal(t, true, tag.BuiltIn)
	})

	t.Run("non-admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, username).Return(database.User{UUID: "testUUID", RoleMask: "persion"}, nil)

		tag, err := tc.CreateTag(ctx, username, req)
		require.NotNil(t, err)
		require.Nil(t, tag)
	})
}

func TestTagComponent_GetTagByID(t *testing.T) {
	ctx := context.TODO()
	username := "testUser"
	t.Run("admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)
		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, username).Return(database.User{UUID: "testUUID", RoleMask: "admin"}, nil)
		tc.mocks.stores.TagMock().EXPECT().FindTagByID(ctx, int64(1)).Return(&database.Tag{ID: int64(1), Name: "test-tag"}, nil)

		tag, err := tc.GetTagByID(ctx, username, int64(1))
		require.Nil(t, err)
		require.Equal(t, int64(1), tag.ID)
		require.Equal(t, "test-tag", tag.Name)
	})

	t.Run("non-admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, username).Return(database.User{UUID: "testUUID", RoleMask: "person"}, nil)

		tag, err := tc.GetTagByID(ctx, username, int64(1))
		require.NotNil(t, err)
		require.Nil(t, tag)
	})
}

func TestTagComponent_UpdateTag(t *testing.T) {
	ctx := context.TODO()

	username := "testUser"

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

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, username).Return(database.User{UUID: "testUUID", RoleMask: "admin"}, nil)
		tc.mocks.stores.TagMock().EXPECT().UpdateTagByID(ctx, &newTag).Return(&newTag, nil)
		tc.mocks.moderationClient.EXPECT().PassTextCheck(ctx, mock.Anything, req.Name).Return(&rpc.CheckResult{
			IsSensitive: false,
		}, nil)

		tag, err := tc.UpdateTag(ctx, username, int64(1), req)
		require.Nil(t, err)
		require.Equal(t, req.Name, tag.Name)
		require.Equal(t, true, tag.BuiltIn)
	})

	t.Run("non-admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, username).Return(database.User{UUID: "testUUID", RoleMask: "persion"}, nil)

		tag, err := tc.UpdateTag(ctx, username, int64(1), req)
		require.NotNil(t, err)
		require.Nil(t, tag)
	})
}

func TestTagComponent_DeleteTag(t *testing.T) {
	ctx := context.TODO()

	username := "testUser"

	t.Run("admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, username).Return(database.User{UUID: "testUUID", RoleMask: "admin"}, nil)
		tc.mocks.stores.TagMock().EXPECT().DeleteTagByID(ctx, int64(1)).Return(nil)

		err := tc.DeleteTag(ctx, username, int64(1))
		require.Nil(t, err)
	})

	t.Run("non-admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, username).Return(database.User{UUID: "testUUID", RoleMask: "persion"}, nil)

		err := tc.DeleteTag(ctx, username, int64(1))
		require.NotNil(t, err)
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

	filter := &types.TagFilter{
		Categories: []string{"c"},
		Scopes:     []types.TagScope{types.DatasetTagScope},
	}
	tc.mocks.stores.TagMock().EXPECT().AllTags(ctx, filter).Return(
		[]*database.Tag{
			{Name: "t1", ID: 2},
		}, nil,
	)
	tc.mocks.stores.RepoMock().EXPECT().TagIDs(ctx, int64(1), "c").Return([]int64{1}, nil)
	tc.mocks.stores.TagMock().EXPECT().UpsertRepoTags(ctx, int64(1), []int64{1}, []int64{2}).Return(nil)

	err := tc.UpdateRepoTagsByCategory(ctx, types.DatasetTagScope, 1, "c", []string{"t1"})
	require.Nil(t, err)
}

func TestTagComponent_AllCategories(t *testing.T) {
	ctx := context.TODO()
	tc := initializeTestTagComponent(ctx, t)

	tc.mocks.stores.TagMock().EXPECT().AllCategories(ctx, types.TagScope("")).Return([]database.TagCategory{}, nil)

	categories, err := tc.AllCategories(ctx)
	require.Nil(t, err)
	require.Equal(t, []database.TagCategory{}, categories)
}

func TestTagComponent_CreateCategory(t *testing.T) {
	ctx := context.TODO()

	t.Run("admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin").Return(database.User{
			Username: "admin",
			RoleMask: "admin",
		}, nil)
		tc.mocks.stores.TagMock().EXPECT().CreateCategory(ctx, database.TagCategory{
			Name:  "test-cate",
			Scope: types.TagScope("test-scope"),
		}).Return(&database.TagCategory{
			ID:    1,
			Name:  "test-cate",
			Scope: "test-scope",
		}, nil)

		category, err := tc.CreateCategory(ctx, "admin", types.CreateCategory{
			Name:  "test-cate",
			Scope: "test-scope",
		})
		require.Nil(t, err)
		require.NotNil(t, category)
	})

	t.Run("user", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
			Username: "user",
			RoleMask: "user",
		}, nil)

		category, err := tc.CreateCategory(ctx, "user", types.CreateCategory{
			Name:  "test-cate",
			Scope: "test-scope",
		})
		require.NotNil(t, err)
		require.Nil(t, category)
	})
}

func TestTagComponent_UpdateCategory(t *testing.T) {
	ctx := context.TODO()

	t.Run("admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin").Return(database.User{
			Username: "admin",
			RoleMask: "admin",
		}, nil)
		tc.mocks.stores.TagMock().EXPECT().UpdateCategory(ctx, database.TagCategory{
			ID:    int64(1),
			Name:  "test-cate",
			Scope: types.TagScope("test-scope"),
		}).Return(&database.TagCategory{
			ID:    1,
			Name:  "test-cate",
			Scope: "test-scope",
		}, nil)

		category, err := tc.UpdateCategory(ctx, "admin", types.UpdateCategory{
			Name:  "test-cate",
			Scope: "test-scope",
		}, int64(1))
		require.Nil(t, err)
		require.NotNil(t, category)
	})

	t.Run("user", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
			Username: "user",
			RoleMask: "user",
		}, nil)

		category, err := tc.UpdateCategory(ctx, "user", types.UpdateCategory{
			Name:  "test-cate",
			Scope: "test-scope",
		}, int64(1))
		require.NotNil(t, err)
		require.Nil(t, category)
	})
}

func TestTagComponent_DeleteCategory(t *testing.T) {
	ctx := context.TODO()

	t.Run("admin", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin").Return(database.User{
			Username: "admin",
			RoleMask: "admin",
		}, nil)
		tc.mocks.stores.TagMock().EXPECT().DeleteCategory(ctx, int64(1)).Return(nil)

		err := tc.DeleteCategory(ctx, "admin", int64(1))
		require.Nil(t, err)
	})

	t.Run("user", func(t *testing.T) {
		tc := initializeTestTagComponent(ctx, t)

		tc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
			Username: "user",
			RoleMask: "user",
		}, nil)

		err := tc.DeleteCategory(ctx, "user", int64(1))
		require.NotNil(t, err)
	})
}
