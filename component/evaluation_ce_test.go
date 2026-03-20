//go:build !saas

package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/membership"
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
		OwnerNamespace:     "test",
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
		c.mocks.deployer.EXPECT().GetSharedModeResourceName(mock.Anything).Return("nvidia.com/gpu").Maybe()
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
		c.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, "test", "", int64(0), mock.Anything).Return(&types.CheckExclusiveResp{}, nil)
		c.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceResource{
			ID:        1,
			Resources: string(resource),
		}, nil)
		c.mocks.deployer.EXPECT().GetSharedModeResourceName(mock.Anything).Return("nvidia.com/gpu").Maybe()
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

	t.Run("create evaluation with invalid model id", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		invalidReq := types.EvaluationReq{
			TaskName:           "test",
			ModelIds:           []string{"badmodelid"},
			Username:           "test",
			ResourceId:         0,
			Datasets:           []string{"opencsg/hellaswag"},
			RuntimeFrameworkId: 1,
		}
		c.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, invalidReq.Username).Return(database.User{
			RoleMask: "admin",
			Username: invalidReq.Username,
			UUID:     invalidReq.Username,
			ID:       1,
		}, nil).Once()
		_, err := c.CreateEvaluation(ctx, invalidReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid model id format: badmodelid")
	})

	t.Run("create evaluation with invalid dataset", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		invalidReq := types.EvaluationReq{
			TaskName:           "test",
			ModelIds:           []string{"opencsg/wukong"},
			Username:           "test",
			ResourceId:         0,
			Datasets:           []string{"baddataset"},
			RuntimeFrameworkId: 1,
		}
		c.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, invalidReq.Username).Return(database.User{
			RoleMask: "admin",
			Username: invalidReq.Username,
			UUID:     invalidReq.Username,
			ID:       1,
		}, nil).Once()
		c.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "opencsg", "wukong").Return(
			&database.Model{
				ID: 1,
				Repository: &database.Repository{
					DefaultBranch: "main",
				},
			}, nil,
		).Once()
		c.mocks.stores.AccessTokenMock().EXPECT().FindByUID(ctx, int64(1)).Return(&database.AccessToken{Token: "foo"}, nil).Once()
		_, err := c.CreateEvaluation(ctx, invalidReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to generate mirror repo ids")
	})

	t.Run("create evaluation with invalid custom dataset", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		invalidReq := types.EvaluationReq{
			TaskName:           "test",
			ModelIds:           []string{"opencsg/wukong"},
			Username:           "test",
			ResourceId:         0,
			CustomDataSets:     []string{"badcustomdataset"},
			RuntimeFrameworkId: 1,
		}
		c.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, invalidReq.Username).Return(database.User{
			RoleMask: "admin",
			Username: invalidReq.Username,
			UUID:     invalidReq.Username,
			ID:       1,
		}, nil).Once()
		c.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "opencsg", "wukong").Return(
			&database.Model{
				ID: 1,
				Repository: &database.Repository{
					DefaultBranch: "main",
				},
			}, nil,
		).Once()
		c.mocks.stores.AccessTokenMock().EXPECT().FindByUID(ctx, int64(1)).Return(&database.AccessToken{Token: "foo"}, nil).Once()
		_, err := c.CreateEvaluation(ctx, invalidReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to generate mirror repo ids")
	})

	t.Run("non_admin_namespace_permission_error", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		req := types.EvaluationReq{
			TaskName:           "test",
			ModelIds:           []string{"opencsg/wukong"},
			Username:           "test",
			OwnerNamespace:     "org1",
			ResourceId:         0,
			Datasets:           []string{"opencsg/hellaswag"},
			RuntimeFrameworkId: 1,
		}
		c.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, req.Username).Return(database.User{
			RoleMask: "",
			Username: req.Username,
			UUID:     req.Username,
			ID:       1,
		}, nil).Once()
		c.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "test", "org1", membership.RoleWrite).Return(false, errors.New("rpc error"))
		_, err := c.CreateEvaluation(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to check namespace permission")
	})
	t.Run("non_admin_namespace_forbidden", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		req := types.EvaluationReq{
			TaskName:           "test",
			ModelIds:           []string{"opencsg/wukong"},
			Username:           "test",
			OwnerNamespace:     "org1",
			ResourceId:         0,
			Datasets:           []string{"opencsg/hellaswag"},
			RuntimeFrameworkId: 1,
		}
		c.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, req.Username).Return(database.User{
			RoleMask: "",
			Username: req.Username,
			UUID:     req.Username,
			ID:       1,
		}, nil).Once()
		c.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "test", "org1", membership.RoleWrite).Return(false, nil)
		_, err := c.CreateEvaluation(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "do not have permission to create evaluation in this namespace")
	})
	t.Run("create evaluation under org uses org billing UUID", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		c.config.Argo.QuotaGPUNumber = "1"
		orgReq := types.EvaluationReq{
			TaskName:           "org-eval",
			ModelIds:           []string{"opencsg/wukong"},
			Username:           "testuser",
			OwnerNamespace:     "org1",
			ResourceId:         0,
			Datasets:           []string{"opencsg/hellaswag"},
			RuntimeFrameworkId: 1,
		}
		c.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "testuser").Return(database.User{
			RoleMask: "",
			Username: "testuser",
			UUID:     "user-uuid",
			ID:       1,
		}, nil).Once()
		c.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "testuser", "org1", membership.RoleWrite).Return(true, nil)
		c.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "opencsg", "wukong").Return(
			&database.Model{
				ID: 1,
				Repository: &database.Repository{
					DefaultBranch: "main",
				},
			}, nil,
		).Maybe()
		c.mocks.stores.AccessTokenMock().EXPECT().FindByUID(ctx, int64(1)).Return(&database.AccessToken{Token: "foo"}, nil)
		c.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.DatasetRepo, "opencsg", "hellaswag").Return(&database.Repository{
			ID:            1,
			DefaultBranch: "main",
			HFPath:        "Rowan/hellaswag",
		}, nil)
		c.mocks.stores.RuntimeFrameworkMock().EXPECT().FindEnabledByID(ctx, int64(1)).Return(&database.RuntimeFramework{
			ID:          1,
			FrameImage:  "lm-evaluation-harness:0.4.6",
			ComputeType: string(types.ResourceTypeGPU),
		}, nil)
		c.mocks.stores.ClusterInfoMock().EXPECT().FindNodeByClusterID(ctx, "").Return([]database.ClusterNode{
			{Name: "node1"},
		}, nil)
		c.mocks.deployer.EXPECT().GetSharedModeResourceName(mock.Anything).Return("nvidia.com/gpu").Maybe()
		c.mocks.components.repo.EXPECT().GetNamespaceBillingUUID(ctx, "org1").Return("org-uuid-123", nil)
		c.mocks.deployer.EXPECT().SubmitEvaluation(ctx, mock.MatchedBy(func(req types.EvaluationReq) bool {
			return req.UserUUID == "org-uuid-123" && req.Username == "org1"
		})).Return(&types.ArgoWorkFlowRes{
			ID:       1,
			TaskName: "org-eval",
		}, nil)

		e, err := c.CreateEvaluation(ctx, orgReq)
		require.Nil(t, err)
		require.NotNil(t, e)
		require.Equal(t, "org-eval", e.TaskName)
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
	t.Run("owner is different from req user and req user has no read permission", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)
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
		c.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "otheruser", "test", membership.RoleRead).Return(false, nil)
		e, err := c.GetEvaluation(ctx, req)
		require.Equal(t, err, errorx.ErrForbidden)
		require.Nil(t, e)
	})
	t.Run("owner is different from req user and req user has read permission", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)
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
		c.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "otheruser", "test", membership.RoleRead).Return(true, nil)
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
	t.Run("owner is different from req user and permission check fails", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)
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
		c.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "otheruser", "test", membership.RoleRead).Return(false, errors.New("rpc error"))
		e, err := c.GetEvaluation(ctx, req)
		require.Nil(t, e)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to check namespace permission")
	})
	t.Run("org member can view org evaluation", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)
		req := types.EvaluationGetReq{
			Username: "orgmember",
			ID:       1,
		}
		c.mocks.stores.WorkflowMock().EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
			ID:       1,
			RepoIds:  []string{"Rowan/hellaswag"},
			Datasets: []string{"Rowan/hellaswag"},
			RepoType: "model",
			Username: "org1",
			TaskName: "org-eval",
			TaskId:   "org-eval-1",
			TaskType: "evaluation",
			Status:   "Succeed",
		}, nil)
		c.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "orgmember", "org1", membership.RoleRead).Return(true, nil)
		c.mocks.stores.DatasetMock().EXPECT().FindByOriginPath(ctx, "Rowan/hellaswag").Return(&database.Dataset{
			Repository: &database.Repository{
				Path: "Rowan/hellaswag",
				Tags: []database.Tag{},
			},
		}, nil)
		e, err := c.GetEvaluation(ctx, req)
		require.NotNil(t, e)
		require.Equal(t, "org-eval", e.TaskName)
		require.Nil(t, err)
	})
	t.Run("non org member cannot view org evaluation", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)
		req := types.EvaluationGetReq{
			Username: "outsider",
			ID:       1,
		}
		c.mocks.stores.WorkflowMock().EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
			ID:       1,
			RepoIds:  []string{"Rowan/hellaswag"},
			Datasets: []string{"Rowan/hellaswag"},
			RepoType: "model",
			Username: "org1",
			TaskName: "org-eval",
			TaskId:   "org-eval-1",
			TaskType: "evaluation",
			Status:   "Succeed",
		}, nil)
		c.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "outsider", "org1", membership.RoleRead).Return(false, nil)
		e, err := c.GetEvaluation(ctx, req)
		require.Equal(t, err, errorx.ErrForbidden)
		require.Nil(t, e)
	})
}

func TestEvaluationComponent_DeleteEvaluation(t *testing.T) {
	t.Run("owner can delete own evaluation", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)

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
	})

	t.Run("org member with write permission can delete org evaluation", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)

		c.mocks.stores.WorkflowMock().EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
			ID:        1,
			Username:  "org1",
			TaskId:    "org-task-1",
			TaskType:  "evaluation",
			Status:    "Succeed",
			ClusterID: "cluster1",
			Namespace: "ns1",
		}, nil)
		c.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "orgmember", "org1", membership.RoleWrite).Return(true, nil)
		req := types.EvaluationDelReq{
			Username: "orgmember",
			ID:       1,
		}
		c.mocks.stores.WorkflowMock().EXPECT().DeleteWorkFlow(ctx, int64(1)).Return(nil)
		c.mocks.deployer.EXPECT().DeleteEvaluation(ctx, mock.Anything).Return(nil)
		err := c.DeleteEvaluation(ctx, req)
		require.Nil(t, err)
	})

	t.Run("user without write permission cannot delete org evaluation", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)

		c.mocks.stores.WorkflowMock().EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
			ID:        1,
			Username:  "org1",
			TaskId:    "org-task-1",
			TaskType:  "evaluation",
			Status:    "Succeed",
			ClusterID: "cluster1",
			Namespace: "ns1",
		}, nil)
		c.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "outsider", "org1", membership.RoleWrite).Return(false, nil)
		req := types.EvaluationDelReq{
			Username: "outsider",
			ID:       1,
		}
		err := c.DeleteEvaluation(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "do not have permission to delete evaluation")
	})

	t.Run("delete permission check error", func(t *testing.T) {
		ctx := context.TODO()
		c := initializeTestEvaluationComponent(ctx, t)

		c.mocks.stores.WorkflowMock().EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
			ID:       1,
			Username: "org1",
			TaskId:   "org-task-1",
		}, nil)
		c.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "orgmember", "org1", membership.RoleWrite).Return(false, errors.New("rpc error"))
		req := types.EvaluationDelReq{
			Username: "orgmember",
			ID:       1,
		}
		err := c.DeleteEvaluation(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to check namespace permission")
	})
}

func TestEvaluationComponent_OrgEvaluations(t *testing.T) {
	ctx := context.TODO()

	t.Run("with current user and read permission", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		req := &types.OrgEvaluationsReq{
			Namespace:   "org1",
			CurrentUser: "user1",
			PageOpts:    types.PageOpts{Page: 1, PageSize: 10},
		}
		c.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "user1", "org1", membership.RoleRead).Return(true, nil)
		c.mocks.stores.WorkflowMock().EXPECT().FindByUsername(ctx, "org1", types.TaskTypeEvaluation, 10, 1).Return([]database.ArgoWorkflow{
			{ID: 1, TaskName: "ev1", Username: "org1", TaskType: "evaluation", Status: "Succeed"},
		}, 1, nil)
		res, total, err := c.OrgEvaluations(ctx, req)
		require.Nil(t, err)
		require.Equal(t, 1, total)
		require.Len(t, res, 1)
		require.Equal(t, "ev1", res[0].TaskName)
		require.Equal(t, "org1", res[0].Username)
	})

	t.Run("without current user skips permission check", func(t *testing.T) {
		c := initializeTestEvaluationComponent(ctx, t)
		req := &types.OrgEvaluationsReq{
			Namespace:   "org1",
			CurrentUser: "",
			PageOpts:    types.PageOpts{Page: 1, PageSize: 10},
		}
		c.mocks.stores.WorkflowMock().EXPECT().FindByUsername(ctx, "org1", types.TaskTypeEvaluation, 10, 1).Return([]database.ArgoWorkflow{}, 0, nil)
		res, total, err := c.OrgEvaluations(ctx, req)
		require.Nil(t, err)
		require.Equal(t, 0, total)
		require.Len(t, res, 0)
	})
}
