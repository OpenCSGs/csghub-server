package component

import (
	"context"
	"fmt"
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

func TestUserComponent_Skills(t *testing.T) {
	testCases := []struct {
		name           string
		ownerExists    bool
		ownerError     error
		userExists     bool
		userError      error
		skills         []database.Skill
		total          int
		skillsError    error
		expectedError  bool
		expectedTotal  int
		expectedSkills []types.Skill
	}{
		{
			name:           "Happy path with skills",
			ownerExists:    true,
			ownerError:     nil,
			userExists:     true,
			userError:      nil,
			skills:         []database.Skill{{ID: 1, Repository: &database.Repository{Name: "foo"}}},
			total:          100,
			skillsError:    nil,
			expectedError:  false,
			expectedTotal:  100,
			expectedSkills: []types.Skill{{ID: 1, Name: "foo"}},
		},
		{
			name:           "Owner does not exist",
			ownerExists:    false,
			ownerError:     nil,
			userExists:     true,
			userError:      nil,
			skills:         nil,
			total:          0,
			skillsError:    nil,
			expectedError:  true,
			expectedTotal:  0,
			expectedSkills: nil,
		},
		{
			name:           "Current user does not exist",
			ownerExists:    true,
			ownerError:     nil,
			userExists:     false,
			userError:      nil,
			skills:         nil,
			total:          0,
			skillsError:    nil,
			expectedError:  true,
			expectedTotal:  0,
			expectedSkills: nil,
		},
		{
			name:           "Error checking owner existence",
			ownerExists:    false,
			ownerError:     fmt.Errorf("owner error"),
			userExists:     true,
			userError:      nil,
			skills:         nil,
			total:          0,
			skillsError:    nil,
			expectedError:  true,
			expectedTotal:  0,
			expectedSkills: nil,
		},
		{
			name:           "Error checking user existence",
			ownerExists:    true,
			ownerError:     nil,
			userExists:     false,
			userError:      fmt.Errorf("user error"),
			skills:         nil,
			total:          0,
			skillsError:    nil,
			expectedError:  true,
			expectedTotal:  0,
			expectedSkills: nil,
		},
		{
			name:           "Error getting skills",
			ownerExists:    true,
			ownerError:     nil,
			userExists:     true,
			userError:      nil,
			skills:         nil,
			total:          0,
			skillsError:    fmt.Errorf("skills error"),
			expectedError:  true,
			expectedTotal:  0,
			expectedSkills: nil,
		},
		{
			name:           "No skills returned",
			ownerExists:    true,
			ownerError:     nil,
			userExists:     true,
			userError:      nil,
			skills:         []database.Skill{},
			total:          0,
			skillsError:    nil,
			expectedError:  false,
			expectedTotal:  0,
			expectedSkills: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.TODO()
			uc := initializeTestUserComponent(ctx, t)

			uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "owner").Return(tc.ownerExists, tc.ownerError)
			
			// Only expect current user check if owner exists and has no error
			if tc.ownerExists && tc.ownerError == nil {
				uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "user").Return(tc.userExists, tc.userError)
				
				// Only expect skill retrieval if both users exist and have no errors
				if tc.userExists && tc.userError == nil {
					uc.mocks.stores.SkillMock().EXPECT().ByUsername(ctx, "owner", 10, 1, true).Return(tc.skills, tc.total, tc.skillsError)
				}
			}

			data, total, err := uc.Skills(ctx, &types.UserSkillsReq{
				Owner:       "owner",
				CurrentUser: "user",
				PageOpts: types.PageOpts{
					Page:     1,
					PageSize: 10,
				},
			})

			if tc.expectedError {
				require.Error(t, err)
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.expectedTotal, total)
				require.Equal(t, tc.expectedSkills, data)
			}
		})
	}

	// Test with different pagination parameters
	t.Run("Different pagination parameters", func(t *testing.T) {
		ctx := context.TODO()
		uc := initializeTestUserComponent(ctx, t)

		uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "owner").Return(true, nil)
		uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "user").Return(true, nil)
		uc.mocks.stores.SkillMock().EXPECT().ByUsername(ctx, "owner", 20, 2, true).Return([]database.Skill{
			{ID: 2, Repository: &database.Repository{Name: "bar"}},
		}, 50, nil)

		data, total, err := uc.Skills(ctx, &types.UserSkillsReq{
			Owner:       "owner",
			CurrentUser: "user",
			PageOpts: types.PageOpts{
				Page:     2,
				PageSize: 20,
			},
		})
		require.Nil(t, err)
		require.Equal(t, 50, total)
		require.Equal(t, []types.Skill{
			{ID: 2, Name: "bar"},
		}, data)
	})
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
	data, total, err := uc.Collections(ctx, &types.UserCollectionReq{
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

	req := &types.UserCollectionReq{
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
	require.Equal(t, []types.DeployRequest{
		{
			Path: "models_foo/bar", Status: "Pending", GitPath: "models_foo/bar", Hardware: `{"memory": "foo"}`,
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
	uc.mocks.stores.WorkflowMock().EXPECT().FindByUsername(ctx, "user", types.TaskTypeEvaluation, 10, 1).Return([]database.ArgoWorkflow{
		{ID: 1},
	}, 100, nil)
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

func TestUserComponent_MCPServers(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "owner").Return(true, nil)
	uc.mocks.stores.UserMock().EXPECT().IsExist(ctx, "user").Return(true, nil)
	uc.mocks.stores.MCPServerMock().EXPECT().ByUsername(ctx, "owner", 10, 1, true).Return([]database.MCPServer{
		{ID: 1, Repository: &database.Repository{Name: "foo"}},
	}, 1, nil)

	data, total, err := uc.MCPServers(ctx, &types.UserMCPsReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 1, total)

	require.Equal(t, []types.MCPServer{
		{ID: 1, Name: "foo"},
	}, data)
}

func TestUserComponent_LikesMCPServers(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	req := &types.UserMCPsReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)

	uc.mocks.stores.MCPServerMock().EXPECT().UserLikes(ctx, int64(1), 10, 1).Return([]database.MCPServer{
		{ID: 1, Repository: &database.Repository{Name: "foo"}},
	}, 100, nil)

	data, total, err := uc.LikesMCPServers(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.MCPServer{
		{ID: 1, Name: "foo"},
	}, data)

}

func TestUserComponent_Finetunes(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)
	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(
		database.User{ID: 1, UUID: "uuid"}, nil,
	)
	uc.mocks.stores.WorkflowMock().EXPECT().FindByUsername(ctx, "user", types.TaskTypeFinetune, 10, 1).Return([]database.ArgoWorkflow{
		{ID: 1},
	}, 100, nil)
	data, total, err := uc.ListFinetunes(ctx, &types.UserDatasetsReq{
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

func TestUserComponent_LikesSkills(t *testing.T) {
	ctx := context.TODO()
	uc := initializeTestUserComponent(ctx, t)

	req := &types.UserMCPsReq{
		Owner:       "owner",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	}
	uc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	uc.mocks.stores.SkillMock().EXPECT().UserLikesSkills(ctx, int64(1), 10, 1).Return([]database.Skill{
		{ID: 1, Repository: &database.Repository{Name: "foo"}},
	}, 100, nil)
	data, total, err := uc.LikesSkills(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Skill{
		{ID: 1, Name: "foo"},
	}, data)
}
