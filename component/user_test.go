package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestUserComponent_Datasets(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "owner").Return(true, nil)
	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "user").Return(true, nil)
	uc.mocks.stores.DatasetMock().EXPECT().ByUsername(ctx, "owner", 10, 1, true).Return([]database.Dataset{
		{ID: 1, Repository: &database.Repository{Name: "foo"}},
	}, 100, nil)

	data, total, err := uc.Datasets(ctx, &types.UserDatasetsReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Dataset{
		{ID: 1, Name: "foo"},
	}, data)

}

func TestUserComponent_Models(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "owner").Return(true, nil)
	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "user").Return(true, nil)
	uc.mocks.stores.ModelMock().EXPECT().ByUsername(ctx, "owner", 10, 1, true).Return([]database.Model{
		{ID: 1, Repository: &database.Repository{Name: "foo"}},
	}, 100, nil)

	data, total, err := uc.Models(ctx, &types.UserDatasetsReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Model{
		{ID: 1, Name: "foo"},
	}, data)
}

func TestUserComponent_Codes(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "owner").Return(true, nil)
	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "user").Return(true, nil)
	uc.mocks.stores.CodeMock().EXPECT().ByUsername(ctx, "owner", 10, 1, true).Return([]database.Code{
		{ID: 1, Repository: &database.Repository{Name: "foo"}},
	}, 100, nil)

	data, total, err := uc.Codes(ctx, &types.UserDatasetsReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Code{
		{ID: 1, Name: "foo"},
	}, data)
}

func TestUserComponent_Spaces(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "owner").Return(true, nil)
	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "user").Return(true, nil)

	uc.mocks.components.space.EXPECT().UserSpaces(ctx, &types.UserSpacesReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		}}).Return([]types.Space{
		{ID: 1, Name: "foo"},
	}, 100, nil)

	data, total, err := uc.Spaces(ctx, &types.UserSpacesReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Space{
		{ID: 1, Name: "foo"},
	}, data)

}

func TestUserComponent_AddLikes(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	var opts []interface{}
	opts = append(opts, database.Columns("id", "repository_type", "path", "user_id", "private"))
	repos := []*database.Repository{
		{ID: 1},
	}
	uc.mocks.stores.RepoMock().EXPECT().FindByIds(ctx, []int64{123}, opts...).Return(repos, nil)
	visiable := []*database.Repository{
		{ID: 2},
	}
	uc.mocks.components.repo.EXPECT().VisiableToUser(ctx, repos, "user").Return(visiable, nil)
	uc.mocks.stores.UserLikesMock().EXPECT().Add(ctx, int64(1), int64(123)).Return(nil)

	err := uc.AddLikes(ctx, &types.UserLikesRequest{
		Username:     "user",
		RepoID:       123,
		CollectionID: 456,
		CurrentUser:  "user",
	})
	require.Nil(t, err)
}

func TestUserComponent_LikesCollection(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	uc.mocks.stores.CollectionMock().EXPECT().ByUserLikes(ctx, int64(1), 10, 1).Return([]database.Collection{
		{ID: 1, Name: "foo"},
	}, 100, nil)
	data, total, err := uc.LikesCollection(ctx, &types.UserSpacesReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Collection{
		{ID: 1, Name: "foo"},
	}, data)
}

func TestUserComponent_Collections(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "owner").Return(true, nil)
	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "user").Return(true, nil)
	uc.mocks.stores.CollectionMock().EXPECT().ByUsername(ctx, "owner", 10, 1, true).Return([]database.Collection{
		{ID: 1, Name: "foo"},
	}, 100, nil)
	data, total, err := uc.Collections(ctx, &types.UserSpacesReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Collection{
		{ID: 1, Name: "foo"},
	}, data)
}

func TestUserComponent_LikeCollection(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	uc.mocks.stores.CollectionMock().EXPECT().FindById(ctx, int64(456)).Return(database.Collection{
		ID: 1, Name: "foo",
	}, nil)
	uc.mocks.stores.UserLikesMock().EXPECT().LikeCollection(ctx, int64(1), int64(456)).Return(nil)
	err := uc.LikeCollection(ctx, &types.UserLikesRequest{
		Username:     "user",
		RepoID:       123,
		CollectionID: 456,
		CurrentUser:  "user",
	})
	require.Nil(t, err)
}

func TestUserComponent_UnLikeCollection(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	uc.mocks.stores.UserLikesMock().EXPECT().UnLikeCollection(ctx, int64(1), int64(456)).Return(nil)
	err := uc.UnLikeCollection(ctx, &types.UserLikesRequest{
		Username:     "user",
		RepoID:       123,
		CollectionID: 456,
		CurrentUser:  "user",
	})
	require.Nil(t, err)
}

func TestUserComponent_DeleteLikes(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	uc.mocks.stores.UserLikesMock().EXPECT().Delete(ctx, int64(1), int64(123)).Return(nil)
	err := uc.DeleteLikes(ctx, &types.UserLikesRequest{
		Username:     "user",
		RepoID:       123,
		CollectionID: 456,
		CurrentUser:  "user",
	})
	require.Nil(t, err)
}

func TestUserComponent_LikesSpaces(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	req := &types.UserSpacesReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	uc.mocks.components.space.EXPECT().UserLikesSpaces(ctx, req, int64(1)).Return([]types.Space{
		{ID: 1, Name: "foo"},
	}, 100, nil)
	data, total, err := uc.LikesSpaces(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Space{
		{ID: 1, Name: "foo"},
	}, data)
}

func TestUserComponent_LikesCodes(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	req := &types.UserDatasetsReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	uc.mocks.stores.CodeMock().EXPECT().UserLikesCodes(ctx, int64(1), 10, 1).Return([]database.Code{
		{ID: 1, Repository: &database.Repository{Name: "foo"}},
	}, 100, nil)
	data, total, err := uc.LikesCodes(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Code{
		{ID: 1, Name: "foo"},
	}, data)
}

func TestUserComponent_LikesModels(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	req := &types.UserDatasetsReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	uc.mocks.stores.ModelMock().EXPECT().UserLikesModels(ctx, int64(1), 10, 1).Return([]database.Model{
		{ID: 1, Repository: &database.Repository{Name: "foo"}},
	}, 100, nil)
	data, total, err := uc.LikesModels(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Model{
		{ID: 1, Name: "foo"},
	}, data)
}

func TestUserComponent_LikesDatasets(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	req := &types.UserDatasetsReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	uc.mocks.stores.DatasetMock().EXPECT().UserLikesDatasets(ctx, int64(1), 10, 1).Return([]database.Dataset{
		{ID: 1, Repository: &database.Repository{Name: "foo"}},
	}, 100, nil)
	data, total, err := uc.LikesDatasets(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Dataset{
		{ID: 1, Name: "foo"},
	}, data)
}

func TestUserComponent_ListServeless(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	req := &types.DeployReq{
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	uc.mocks.components.repo.EXPECT().IsAdminRole(database.User{ID: 1}).Return(true)
	uc.mocks.stores.DeployTaskMock().EXPECT().ListServerless(ctx, *req).Return([]database.Deploy{
		{
			SvcName: "svc", ClusterID: "cluster", SKU: "sku",
			GitPath: "models_foo/bar", Hardware: `{"memory": "foo"}`,
			RepoID: 123,
		},
	}, 100, nil)

	data, total, err := uc.ListServerless(ctx, *req)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.DeployRepo{
		{
			Path: "models_foo/bar", Status: "Stopped", GitPath: "models_foo/bar", Hardware: `{"memory": "foo"}`,
			RepoID: 123, SvcName: "svc", ClusterID: "cluster", SKU: "sku",
		},
	}, data)

}

func TestUserComponent_GetUserByName(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(
		database.User{ID: 1, UUID: "uuid"}, nil,
	)
	user, err := uc.GetUserByName(ctx, "user")
	require.Nil(t, err)
	require.Equal(t, "uuid", user.UUID)
}

func TestUserComponent_Prompts(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "owner").Return(true, nil)
	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "user").Return(true, nil)
	uc.mocks.stores.PromptMock().EXPECT().ByUsername(ctx, "owner", 10, 1, true).Return([]database.Prompt{
		{ID: 1, Repository: &database.Repository{Name: "foo"}},
	}, 100, nil)

	data, total, err := uc.Prompts(ctx, &types.UserDatasetsReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)

	require.Equal(t, []types.PromptRes{
		{ID: 1, Name: "foo"},
	}, data)
}

func TestUserComponent_Evaluations(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)
	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(
		database.User{ID: 1, UUID: "uuid"}, nil,
	)
	uc.mocks.deployer.EXPECT().ListEvaluations(ctx, "user", 10, 1).Return(&types.ArgoWorkFlowListRes{
		List:  []types.ArgoWorkFlowRes{{ID: 1}},
		Total: 100,
	}, nil)
	data, total, err := uc.Evaluations(ctx, &types.UserDatasetsReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.ArgoWorkFlowRes{{ID: 1}}, data)
}
