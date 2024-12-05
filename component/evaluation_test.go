package component

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	mock_deploy "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func NewTestEvaluationComponent(deployer deploy.Deployer, stores *tests.MockStores, ac AccountingComponent) EvaluationComponent {
	cfg := &config.Config{}
	cfg.Argo.QuotaGPUNumber = "1"
	return &evaluationComponentImpl{
		deployer:           deployer,
		config:             cfg,
		userStore:          stores.User,
		modelStore:         stores.Model,
		datasetStore:       stores.Dataset,
		mirrorStore:        stores.Mirror,
		spaceResourceStore: stores.SpaceResource,
		tokenStore:         stores.AccessToken,
		rtfm:               stores.RuntimeFramework,
		ac:                 ac,
	}
}

func TestEvaluationComponent_CreateEvaluation(t *testing.T) {
	req := types.EvaluationReq{
		TaskName:           "test",
		ModelId:            "opencsg/wukong",
		Username:           "test",
		ResourceId:         0,
		Datasets:           []string{"opencsg/hellaswag"},
		RuntimeFrameworkId: 1,
	}
	ctx := context.TODO()
	req2 := types.EvaluationReq{
		UserUUID:           "test",
		TaskName:           "test",
		ModelId:            "opencsg/wukong",
		Username:           "test",
		ResourceId:         0,
		Datasets:           []string{"Rowan/hellaswag"},
		RuntimeFrameworkId: 1,
		Hardware: types.HardWare{
			Gpu: types.GPU{
				Num:          "1",
				ResourceName: "nvidia.com/gpu",
			},
			Cpu: types.CPU{
				Num: "8",
			},
			Memory: "32Gi",
		},
		Image:    "lm-evaluation-harness:0.4.6",
		RepoType: "model",
		TaskType: "evaluation",
		Token:    "foo",
	}
	t.Run("create evaluation without resource id", func(t *testing.T) {
		deployerMock := &mock_deploy.MockDeployer{}
		stores := tests.NewMockStores(t)
		ac := &mock_component.MockAccountingComponent{}
		c := NewTestEvaluationComponent(deployerMock, stores, ac)
		stores.UserMock().EXPECT().FindByUsername(ctx, req.Username).Return(database.User{
			RoleMask: "admin",
			Username: req.Username,
			UUID:     req.Username,
			ID:       1,
		}, nil).Once()
		stores.ModelMock().EXPECT().FindByPath(ctx, "opencsg", "wukong").Return(
			&database.Model{
				ID: 1,
			}, nil,
		).Maybe()
		stores.MirrorMock().EXPECT().FindByRepoPath(ctx, types.DatasetRepo, "opencsg", "hellaswag").Return(&database.Mirror{
			SourceRepoPath: "Rowan/hellaswag",
		}, nil)
		stores.AccessTokenMock().EXPECT().FindByUID(ctx, int64(1)).Return(&database.AccessToken{Token: "foo"}, nil)
		stores.RuntimeFrameworkMock().EXPECT().FindEnabledByID(ctx, int64(1)).Return(&database.RuntimeFramework{
			ID:         1,
			FrameImage: "lm-evaluation-harness:0.4.6",
		}, nil)
		deployerMock.EXPECT().SubmitEvaluation(ctx, req2).Return(&types.ArgoWorkFlowRes{
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
		deployerMock := &mock_deploy.MockDeployer{}
		stores := tests.NewMockStores(t)
		ac := &mock_component.MockAccountingComponent{}
		c := NewTestEvaluationComponent(deployerMock, stores, ac)
		stores.UserMock().EXPECT().FindByUsername(ctx, req.Username).Return(database.User{
			RoleMask: "admin",
			Username: req.Username,
			UUID:     req.Username,
			ID:       1,
		}, nil).Once()
		stores.ModelMock().EXPECT().FindByPath(ctx, "opencsg", "wukong").Return(
			&database.Model{
				ID: 1,
			}, nil,
		).Maybe()
		stores.MirrorMock().EXPECT().FindByRepoPath(ctx, types.DatasetRepo, "opencsg", "hellaswag").Return(&database.Mirror{
			SourceRepoPath: "Rowan/hellaswag",
		}, nil)
		stores.AccessTokenMock().EXPECT().FindByUID(ctx, int64(1)).Return(&database.AccessToken{Token: "foo"}, nil)
		stores.RuntimeFrameworkMock().EXPECT().FindEnabledByID(ctx, int64(1)).Return(&database.RuntimeFramework{
			ID:         1,
			FrameImage: "lm-evaluation-harness:0.4.6",
		}, nil)
		resource, err := json.Marshal(req2.Hardware)
		require.Nil(t, err)
		stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceResource{
			ID:        1,
			Resources: string(resource),
		}, nil)
		deployerMock.EXPECT().SubmitEvaluation(ctx, req2).Return(&types.ArgoWorkFlowRes{
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
	deployerMock := &mock_deploy.MockDeployer{}
	stores := tests.NewMockStores(t)
	ac := &mock_component.MockAccountingComponent{}
	c := NewTestEvaluationComponent(deployerMock, stores, ac)
	req := types.EvaluationGetReq{
		Username: "test",
	}
	ctx := context.TODO()
	deployerMock.EXPECT().GetEvaluation(ctx, req).Return(&types.ArgoWorkFlowRes{
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
	stores.DatasetMock().EXPECT().ListByPath(ctx, []string{"Rowan/hellaswag"}).Return([]database.Dataset{
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
	deployerMock := &mock_deploy.MockDeployer{}
	stores := tests.NewMockStores(t)
	ac := &mock_component.MockAccountingComponent{}
	c := NewTestEvaluationComponent(deployerMock, stores, ac)
	req := types.EvaluationDelReq{
		Username: "test",
	}
	ctx := context.TODO()
	deployerMock.EXPECT().DeleteEvaluation(ctx, req).Return(nil)
	err := c.DeleteEvaluation(ctx, req)
	require.Nil(t, err)
}
