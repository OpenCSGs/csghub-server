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
		Scope:    database.TagScope(req.Scope),
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
		Scope:    database.TagScope(req.Scope),
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
