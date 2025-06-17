package component

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestEvaluationComponent_CreateEvaluation(t *testing.T) {
	req := types.EvaluationReq{
		TaskName:           "test",
		Username:           "test",
		ResourceId:         0,
		Datasets:           []string{"opencsg/hellaswag"},
		RuntimeFrameworkId: 1,
		ModelIds:           []string{"opencsg/wukong"},
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
		c.mocks.deployer.EXPECT().SubmitEvaluation(ctx, req2).Return(&types.ArgoWorkFlowRes{
			ID:       1,
			TaskName: "test",
		}, nil)
		e, err := c.CreateEvaluation(ctx, req)
		require.NotNil(t, e)
		require.Equal(t, "test", e.TaskName)
		require.Nil(t, err)
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
		c.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceResource{
			ID:        1,
			Resources: string(resource),
		}, nil)
		c.mocks.deployer.EXPECT().SubmitEvaluation(ctx, req2).Return(&types.ArgoWorkFlowRes{

			ID:       1,
			TaskName: "test",
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
	}
	c.mocks.deployer.EXPECT().GetEvaluation(ctx, req).Return(&types.ArgoWorkFlowRes{
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
	c.mocks.stores.DatasetMock().EXPECT().ListByPath(ctx, []string{"Rowan/hellaswag"}).Return([]database.Dataset{
		{
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
		},
	}, nil)
	e, err := c.GetEvaluation(ctx, req)
	require.NotNil(t, e)
	require.Equal(t, "test", e.TaskName)
	require.Nil(t, err)
}

func TestEvaluationComponent_DeleteEvaluation(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestEvaluationComponent(ctx, t)
	c.config.Argo.QuotaGPUNumber = "1"
	req := types.EvaluationDelReq{
		Username: "test",
	}
	c.mocks.deployer.EXPECT().DeleteEvaluation(ctx, req).Return(nil)
	err := c.DeleteEvaluation(ctx, req)
	require.Nil(t, err)
}
