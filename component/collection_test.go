package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mock_rebac "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestCollectionComponent_GetCollections(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCollectionComponent(ctx, t)

	filter := &types.CollectionFilter{Search: "foo"}
	cc.mocks.stores.CollectionMock().EXPECT().GetCollections(ctx, filter, 10, 1, true).Return(
		[]database.Collection{{Name: "n"}}, 100, nil,
	)
	data, total, err := cc.GetCollections(ctx, filter, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Collection{{Name: "n"}}, data)
}

func TestCollectionComponent_CreateCollection(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCollectionComponent(ctx, t)

	cc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		Username: "user",
	}, nil)
	cc.mocks.stores.CollectionMock().EXPECT().CreateCollection(ctx, database.Collection{
		Username:    "user",
		Name:        "n",
		Nickname:    "nn",
		Description: "d",
	}).Return(&database.Collection{}, nil)

	r, err := cc.CreateCollection(ctx, types.CreateCollectionReq{
		Name:        "n",
		Nickname:    "nn",
		Description: "d",
		Username:    "user",
	})
	require.Nil(t, err)
	require.Equal(t, &database.Collection{}, r)
}

func TestCollectionComponent_GetCollection(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCollectionComponent(ctx, t)

	repos := []database.Repository{
		{ID: 1, RepositoryType: types.SpaceRepo, Path: "r1/foo"},
	}
	cc.mocks.stores.CollectionMock().EXPECT().GetCollection(ctx, int64(1)).Return(
		&database.Collection{Username: "user", Namespace: "user", Repositories: repos}, nil,
	)
	cc.mocks.stores.CollectionMock().EXPECT().GetCollection(ctx, int64(2)).Return(
		&database.Collection{Namespace: "ns", Repositories: repos}, nil,
	)
	// Mock the new GetCollectionRepositoriesWithRemarks method
	cc.mocks.stores.CollectionMock().EXPECT().GetCollectionRepos(ctx, int64(1)).Return(
		[]database.CollectionRepository{
			{RepositoryID: 1, Remark: "Test remark"},
		}, nil,
	)
	cc.mocks.stores.CollectionMock().EXPECT().GetCollectionRepos(ctx, int64(2)).Return(
		[]database.CollectionRepository{
			{RepositoryID: 1, Remark: "Another remark"},
		}, nil,
	)
	cc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		Username: "user",
		UUID:     "user-uuid",
		Avatar:   "aaa",
	}, nil)
	cc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{UUID: "namespace-uuid"}, nil)
	cc.mocks.stores.OrgMock().EXPECT().FindByPath(ctx, "ns").Return(database.Organization{
		Logo: "logo",
	}, nil)
	authorizer := cc.rebac.(*mock_rebac.MockAuthorizer)
	for _, permission := range []rebac.Permission{
		rebac.NamespaceCanRead,
		rebac.NamespaceCanWrite,
		rebac.NamespaceCanAdmin,
	} {
		authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
			Subject:  rebac.UserSubject("user-uuid"),
			Relation: permission,
			Object:   rebac.NamespaceObject("namespace-uuid"),
		}).Return(rebac.Decision{Allowed: true}, nil).Once()
	}
	cc.mocks.stores.UserLikesMock().EXPECT().IsExistCollection(ctx, "user", mock.Anything).Return(true, nil)
	cc.mocks.components.space.EXPECT().Status(ctx, "r1", "foo").Return("", "go", nil)

	col, err := cc.GetCollection(ctx, "user", 1)
	require.Nil(t, err)
	require.Equal(t, &types.Collection{
		Username:  "user",
		Namespace: "user",
		UserLikes: true,
		CanWrite:  true,
		CanManage: true,
		Avatar:    "aaa",
		Repositories: []types.CollectionRepository{
			{ID: 1, RepositoryType: types.SpaceRepo, Path: "r1/foo", Status: "go", Remark: "Test remark"},
		},
	}, col)
	col, err = cc.GetCollection(ctx, "user", 2)
	require.Nil(t, err)
	require.Equal(t, &types.Collection{
		Namespace: "ns",
		UserLikes: true,
		CanWrite:  true,
		CanManage: true,
		Avatar:    "logo",
		Repositories: []types.CollectionRepository{
			{ID: 1, RepositoryType: types.SpaceRepo, Path: "r1/foo", Status: "go", Remark: "Another remark"},
		},
	}, col)
}

func TestCollectionComponent_GetPublicRepos(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCollectionComponent(ctx, t)

	repos := []types.CollectionRepository{
		{RepositoryType: types.SpaceRepo, Path: "r1/foo", Private: true},
		{RepositoryType: types.SpaceRepo, Path: "r1/foo", Private: false},
	}
	r := cc.GetPublicRepos(types.Collection{Repositories: repos})
	require.Equal(t, []types.CollectionRepository{
		{RepositoryType: types.SpaceRepo, Path: "r1/foo", Private: false},
	}, r)
}

func TestCollectionComponent_UpdateCollection(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCollectionComponent(ctx, t)

	cc.mocks.stores.CollectionMock().EXPECT().GetCollection(ctx, int64(1)).Return(
		&database.Collection{}, nil,
	)
	cc.mocks.stores.CollectionMock().EXPECT().UpdateCollection(ctx, mock.Anything).RunAndReturn(func(ctx context.Context, c database.Collection) (*database.Collection, error) {
		require.Equal(t, c.Name, "n")
		require.True(t, c.Private)
		return &database.Collection{}, nil
	})

	r, err := cc.UpdateCollection(ctx, types.CreateCollectionReq{
		ID:      1,
		Name:    "n",
		Private: true,
	})
	require.Nil(t, err)
	require.Equal(t, &database.Collection{}, r)
}

func TestCollectionComponent_DeleteCollection(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCollectionComponent(ctx, t)

	cc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		ID:       2,
		Username: "user",
	}, nil)
	cc.mocks.stores.CollectionMock().EXPECT().DeleteCollection(ctx, int64(1), int64(2)).Return(nil)

	err := cc.DeleteCollection(ctx, 1, "user")
	require.Nil(t, err)

}

func TestCollectionComponent_AddReposToCollection(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCollectionComponent(ctx, t)

	cc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		ID:       2,
		Username: "user",
	}, nil)
	cc.mocks.stores.CollectionMock().EXPECT().GetCollection(ctx, int64(1)).Return(
		&database.Collection{UserID: 2}, nil,
	)
	cc.mocks.stores.CollectionMock().EXPECT().AddCollectionRepos(ctx, []database.CollectionRepository{
		{CollectionID: 1, RepositoryID: 1},
		{CollectionID: 1, RepositoryID: 2},
	}).Return(nil)

	err := cc.AddReposToCollection(ctx, types.UpdateCollectionReposReq{
		RepoIDs:  []int64{1, 2},
		Username: "user",
		ID:       1,
	})
	require.Nil(t, err)

}

func TestCollectionComponent_AddReposToCollectionWithRemarks(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCollectionComponent(ctx, t)

	cc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		ID:       2,
		Username: "user",
	}, nil)
	cc.mocks.stores.CollectionMock().EXPECT().GetCollection(ctx, int64(1)).Return(
		&database.Collection{UserID: 2}, nil,
	)
	cc.mocks.stores.CollectionMock().EXPECT().AddCollectionRepos(ctx, []database.CollectionRepository{
		{CollectionID: 1, RepositoryID: 1, Remark: "test remark"},
		{CollectionID: 1, RepositoryID: 2, Remark: "test remark"},
	}).Return(nil)

	err := cc.AddReposToCollection(ctx, types.UpdateCollectionReposReq{
		RepoIDs:  []int64{1, 2},
		Username: "user",
		ID:       1,
		Remarks: map[int64]string{
			1: "test remark",
			2: "test remark",
		},
	})
	require.Nil(t, err)
}

func TestCollectionComponent_RemoveReposFromCollection(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCollectionComponent(ctx, t)

	cc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		ID:       2,
		Username: "user",
	}, nil)
	cc.mocks.stores.CollectionMock().EXPECT().GetCollection(ctx, int64(1)).Return(
		&database.Collection{UserID: 2}, nil,
	)
	cc.mocks.stores.CollectionMock().EXPECT().RemoveCollectionRepos(ctx, []database.CollectionRepository{
		{CollectionID: 1, RepositoryID: 1},
		{CollectionID: 1, RepositoryID: 2},
	}).Return(nil)

	err := cc.RemoveReposFromCollection(ctx, types.UpdateCollectionReposReq{
		RepoIDs:  []int64{1, 2},
		Username: "user",
		ID:       1,
	})
	require.Nil(t, err)

}

func TestCollectionComponent_OrgCollections(t *testing.T) {
	tests := []struct {
		name       string
		allowed    bool
		checkErr   error
		onlyPublic bool
	}{
		{name: "read allowed", allowed: true, onlyPublic: false},
		{name: "read denied", allowed: false, onlyPublic: true},
		{name: "check failed", checkErr: errors.New("check failed"), onlyPublic: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.TODO()
			cc := initializeTestCollectionComponent(ctx, t)
			cc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{UUID: "user-uuid"}, nil)
			cc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{UUID: "namespace-uuid"}, nil)
			cc.rebac.(*mock_rebac.MockAuthorizer).EXPECT().Check(ctx, rebac.CheckRequest{
				Subject:  rebac.UserSubject("user-uuid"),
				Relation: rebac.NamespaceCanRead,
				Object:   rebac.NamespaceObject("namespace-uuid"),
			}).Return(rebac.Decision{Allowed: tc.allowed}, tc.checkErr).Once()
			cc.mocks.stores.CollectionMock().EXPECT().ByUserOrgs(ctx, "ns", 10, 1, tc.onlyPublic).Return([]database.Collection{
				{Name: "col"},
			}, 100, nil)

			cols, total, err := cc.OrgCollections(ctx, &types.OrgDatasetsReq{
				Namespace: "ns", CurrentUser: "user",
				PageOpts: types.PageOpts{Page: 1, PageSize: 10},
			})
			require.NoError(t, err)
			require.Equal(t, 100, total)
			require.Equal(t, []types.Collection{{Name: "col"}}, cols)
		})
	}
}

func TestCollectionComponent_UpdateCollectionRepo(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCollectionComponent(ctx, t)

	cc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		ID:       2,
		Username: "user",
	}, nil)
	cc.mocks.stores.CollectionMock().EXPECT().GetCollection(ctx, int64(1)).Return(
		&database.Collection{UserID: 2}, nil,
	)
	cc.mocks.stores.CollectionMock().EXPECT().UpdateCollectionRepo(ctx, database.CollectionRepository{
		CollectionID: 1,
		RepositoryID: 1,
		Remark:       "test remark",
	}).Return(nil)

	err := cc.UpdateCollectionRepo(ctx, types.UpdateCollectionRepoReq{
		Username: "user",
		ID:       1,
		RepoID:   1,
		Remark:   "test remark",
	})
	require.Nil(t, err)
}
