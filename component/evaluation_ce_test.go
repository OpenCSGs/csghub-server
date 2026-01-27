//go:build !saas

package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestEvaluationComponent_CreateEvaluation(t *testing.T) {
	req := types.EvaluationReq{
		TaskName:           "test",
		ModelIds:           []string{"opencsg/wukong"},
		Username:           "test",
		ResourceId:         0,
		Datasets:           []string{"opencsg/hellaswag"},
		RuntimeFrameworkId: 1,
	}
	ctx := context.TODO()
	req2 := types.EvaluationReq{
		UserUUID:           "test",
		TaskName:           "test",
		ModelIds:           []string{"opencsg/wukong"},
		Username:           "test",
		ResourceId:         0,
		Datasets:           []string{"Rowan/hellaswag"},
		DatasetRevisions:   []string{"main"},
		RuntimeFrameworkId: 1,
		Revisions:          []string{"main"},
		Hardware: types.HardWare{
			Gpu: types.Processor{
				Num:          "1",
				ResourceName: "nvidia.com/gpu",
			},
			Cpu: types.CPU{
				Num: "4",
			},
			Memory: "32Gi",
		},
		Image:    "lm-evaluation-harness:0.4.6",
		RepoType: "model",
		TaskType: "evaluation",
		Token:    "foo",
		Nodes:    []types.Node{{Name: "node1", EnableVXPU: false}},
	}
	t.Run("create evaluation without resource id", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		c.config.Argo.QuotaGPUNumber = "1"
		c.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, req.Username).Return(database.User{
			RoleMask: "admin",
			Username: req.Username,
			UUID:     req.Username,
			ID:       1,
		}, nil).Once()
		c.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "opencsg", "wukong").Return(
			&database.Model{
				ID: 1,
				Repository: &database.Repository{
					DefaultBranch: "main",
				},
			}, nil,
		).Maybe()
		c.mocks.stores.RuntimeFrameworkMock().EXPECT().FindEnabledByID(ctx, int64(1)).Return(&database.RuntimeFramework{
			ID:          1,
			FrameImage:  "lm-evaluation-harness:0.4.6",
			ComputeType: string(types.ResourceTypeGPU),
		}, nil)
		c.mocks.stores.AccessTokenMock().EXPECT().FindByUID(ctx, int64(1)).Return(&database.AccessToken{Token: "foo"}, nil)
		c.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.DatasetRepo, "opencsg", "hellaswag").Return(&database.Repository{
			ID:            1,
			DefaultBranch: "main",
			HFPath:        "Rowan/hellaswag",
		}, nil)
		req2.ResourceName = "1 GPU · 4 vCPU · 32Gi"
		c.mocks.deployer.EXPECT().SubmitEvaluation(ctx, req2).Return(&types.ArgoWorkFlowRes{
			ID:       1,
			TaskName: "test",
		}, nil)
		req.ResourceName = "1 GPU · 4 vCPU · 32Gi"

		c.mocks.stores.ClusterInfoMock().EXPECT().FindNodeByClusterID(ctx, req.ClusterID).Return([]database.ClusterNode{
			{
				Name: "node1",
			},
		}, nil)

		e, err := c.CreateEvaluation(ctx, req)
		require.NotNil(t, e)
		require.Equal(t, "test", e.TaskName)
		require.Nil(t, err)
		req2.ResourceName = ""
		req.ResourceName = ""
	})
	t.Run("create evaluation with resource id", func(t *testing.T) {
		req.ResourceId = 1
		req2.ResourceId = 1
		c := initializeTestEvaluationComponent(ctx, t)
		c.config.Argo.QuotaGPUNumber = "1"
		c.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, req.Username).Return(database.User{
			RoleMask: "admin",
			Username: req.Username,
			UUID:     req.Username,
			ID:       1,
		}, nil).Once()
		c.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "opencsg", "wukong").Return(
			&database.Model{
				ID: 1,
				Repository: &database.Repository{
					DefaultBranch: "main",
				},
			}, nil,
		).Maybe()
		c.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.DatasetRepo, "opencsg", "hellaswag").Return(&database.Repository{
			ID:            1,
			DefaultBranch: "main",
			HFPath:        "Rowan/hellaswag",
		}, nil)
		c.mocks.stores.AccessTokenMock().EXPECT().FindByUID(ctx, int64(1)).Return(&database.AccessToken{Token: "foo"}, nil)
		c.mocks.stores.RuntimeFrameworkMock().EXPECT().FindEnabledByID(ctx, int64(1)).Return(&database.RuntimeFramework{
			ID:          1,
			FrameImage:  "lm-evaluation-harness:0.4.6",
			ComputeType: string(types.ResourceTypeGPU),
		}, nil)
		resource, err := json.Marshal(req2.Hardware)
		require.Nil(t, err)
		c.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, "test", "", int64(0), mock.Anything).Return(nil)
		c.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceResource{
			ID:        1,
			Resources: string(resource),
		}, nil)
		c.mocks.deployer.EXPECT().SubmitEvaluation(ctx, req2).Return(&types.ArgoWorkFlowRes{
			ID:       1,
			TaskName: "test",
		}, nil)
		req.ResourceName = "1 GPU · 4 vCPU · 32Gi"

		c.mocks.stores.ClusterInfoMock().EXPECT().FindNodeByClusterID(ctx, req.ClusterID).Return([]database.ClusterNode{
			{
				Name: "node1",
			},
		}, nil)

		e, err := c.CreateEvaluation(ctx, req)
		require.NotNil(t, e)
		require.Equal(t, "test", e.TaskName)
		require.Nil(t, err)
	})
}

func TestEvaluationComponent_GetEvaluation(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestEvaluationComponent(ctx, t)
	c.config.Argo.QuotaGPUNumber = "1"
	req := types.EvaluationGetReq{
		Username: "test",
		ID:       1,
	}
	c.mocks.stores.WorkflowMock().EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
		ID:       1,
		RepoIds:  []string{"Rowan/hellaswag"},
		Datasets: []string{"Rowan/hellaswag"},
		RepoType: "model",
		Username: "test",
		TaskName: "test",
		TaskId:   "test",
		TaskType: "evaluation",
		Status:   "Succeed",
	}, nil)
	c.mocks.stores.DatasetMock().EXPECT().FindByOriginPath(ctx, "Rowan/hellaswag").Return(&database.Dataset{
		Repository: &database.Repository{
			Path: "Rowan/hellaswag",
			Tags: []database.Tag{
				{
					Name:     "test",
					Category: "test",
					Group:    "test",
					Scope:    "test",
					BuiltIn:  true,
				},
			},
		},
	}, nil)
	e, err := c.GetEvaluation(ctx, req)
	require.NotNil(t, e)
	require.Equal(t, "test", e.TaskName)
	require.Nil(t, err)
}

func TestEvaluationComponent_GetEvaluation_AccessControl(t *testing.T) {
	t.Run("owner is req user", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)
		c.config.Argo.QuotaGPUNumber = "1"
		req := types.EvaluationGetReq{
			Username: "test",
			ID:       1,
		}
		c.mocks.stores.WorkflowMock().EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
			ID:       1,
			RepoIds:  []string{"Rowan/hellaswag"},
			Datasets: []string{"deleted/dataset"},
			RepoType: "model",
			Username: "test",
			TaskName: "test",
			TaskId:   "test",
			TaskType: "evaluation",
			Status:   "Succeed",
		}, nil)
		c.mocks.stores.DatasetMock().EXPECT().FindByOriginPath(ctx, "deleted/dataset").Return(nil, sql.ErrNoRows)
		e, err := c.GetEvaluation(ctx, req)
		require.NotNil(t, e)
		require.Equal(t, "test", e.TaskName)
		require.Nil(t, err)
		require.Len(t, e.Datasets, 1)
		require.True(t, e.Datasets[0].Deleted)
		require.Equal(t, "deleted/dataset", e.Datasets[0].RepoId)
	})
	t.Run("owner is different from req user and req user is not admin", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)
		c.config.Argo.QuotaGPUNumber = "1"
		req := types.EvaluationGetReq{
			Username: "otheruser",
			ID:       1,
		}
		c.mocks.stores.WorkflowMock().EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
			ID:       1,
			RepoIds:  []string{"Rowan/hellaswag"},
			Datasets: []string{"Rowan/hellaswag"},
			RepoType: "model",
			Username: "test",
			TaskName: "test",
			TaskId:   "test",
			TaskType: "evaluation",
			Status:   "Succeed",
		}, nil)
		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, "otheruser").Return(&types.User{
			Roles:    []string{"user"},
			Username: "otheruser",
			UUID:     "otheruser",
			ID:       2,
		}, nil)
		e, err := c.GetEvaluation(ctx, req)
		require.Equal(t, err, errorx.ErrForbidden)
		require.Nil(t, e)
	})
	t.Run("owner is different from req user and req user is admin", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)
		c.config.Argo.QuotaGPUNumber = "1"
		req := types.EvaluationGetReq{
			Username: "otheruser",
			ID:       1,
		}
		c.mocks.stores.WorkflowMock().EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
			ID:       1,
			RepoIds:  []string{"Rowan/hellaswag"},
			Datasets: []string{"Rowan/hellaswag"},
			RepoType: "model",
			Username: "test",
			TaskName: "test",
			TaskId:   "test",
			TaskType: "evaluation",
			Status:   "Succeed",
		}, nil)
		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, "otheruser").Return(&types.User{
			Roles:    []string{"admin"},
			Username: "otheruser",
			UUID:     "otheruser",
			ID:       2,
		}, nil)
		c.mocks.stores.DatasetMock().EXPECT().FindByOriginPath(ctx, "Rowan/hellaswag").Return(&database.Dataset{
			Repository: &database.Repository{
				Path: "Rowan/hellaswag",
				Tags: []database.Tag{
					{
						Name:     "test",
						Category: "test",
						Group:    "test",
						Scope:    "test",
						BuiltIn:  true,
					},
				},
			},
		}, nil)
		e, err := c.GetEvaluation(ctx, req)
		require.NotNil(t, e)
		require.Equal(t, "test", e.TaskName)
		require.Nil(t, err)
		require.Len(t, e.Datasets, 1)
		require.False(t, e.Datasets[0].Deleted)
		require.Equal(t, "Rowan/hellaswag", e.Datasets[0].RepoId)
	})
	t.Run("owner is different from req user and get user info fails", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)
		c.config.Argo.QuotaGPUNumber = "1"
		req := types.EvaluationGetReq{
			Username: "otheruser",
			ID:       1,
		}
		c.mocks.stores.WorkflowMock().EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
			ID:       1,
			RepoIds:  []string{"Rowan/hellaswag"},
			Datasets: []string{"Rowan/hellaswag"},
			RepoType: "model",
			Username: "test",
			TaskName: "test",
			TaskId:   "test",
			TaskType: "evaluation",
			Status:   "Succeed",
		}, nil)
		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, "otheruser").Return(nil, errorx.ErrRemoteServiceFail)
		e, err := c.GetEvaluation(ctx, req)
		require.Nil(t, e)
		require.Error(t, err)
		require.Contains(t, err.Error(), errorx.ErrRemoteServiceFail.Error())
	})
}

func TestEvaluationComponent_DeleteEvaluation(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestEvaluationComponent(ctx, t)
	c.config.Argo.QuotaGPUNumber = "1"

	c.mocks.stores.WorkflowMock().EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
		ID:        1,
		RepoIds:   []string{"Rowan/hellaswag"},
		Datasets:  []string{"Rowan/hellaswag"},
		RepoType:  "model",
		Username:  "test",
		TaskName:  "test",
		TaskId:    "test",
		TaskType:  "evaluation",
		Status:    "Succeed",
		ClusterID: "test",
		Namespace: "test",
	}, nil)
	req := types.EvaluationDelReq{
		Username:  "test",
		ID:        1,
		TaskID:    "test",
		ClusterID: "test",
		Namespace: "test",
	}
	c.mocks.stores.WorkflowMock().EXPECT().DeleteWorkFlow(ctx, int64(1)).Return(nil)
	c.mocks.deployer.EXPECT().DeleteEvaluation(ctx, req).Return(nil)
	err := c.DeleteEvaluation(ctx, req)
	require.Nil(t, err)
}
