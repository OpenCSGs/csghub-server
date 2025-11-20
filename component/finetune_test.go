package component

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"

	mockdeploy "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockComps "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
)

func NewTestFinetuneComponent(config *config.Config,
	deployer deploy.Deployer, userStore database.UserStore, modelStore database.ModelStore,
	spaceResStore database.SpaceResourceStore, datasetStore database.DatasetStore,
	mirrorStore database.MirrorStore, tokenStore database.AccessTokenStore,
	repoStore database.RepoStore, frameStore database.RuntimeFrameworksStore,
	argoStore database.ArgoWorkFlowStore,
	acctComp AccountingComponent, repoComp RepoComponent) FinetuneComponent {
	c := &finetuneComponentImpl{}
	c.deployer = deployer
	c.userStore = userStore
	c.modelStore = modelStore
	c.spaceResourceStore = spaceResStore
	c.datasetStore = datasetStore
	c.mirrorStore = mirrorStore
	c.tokenStore = tokenStore
	c.repoStore = repoStore
	c.runtimeFrameworkStore = frameStore
	c.workflowStore = argoStore
	c.config = config
	c.repoComponent = repoComp
	c.accountingComponent = acctComp
	return c
}

func TestEvaluationComponent_CreateFinetune(t *testing.T) {
	req := types.FinetuneReq{
		Username:           "testuser",
		TaskName:           "testtask",
		RuntimeFrameworkId: 1,
		ResourceId:         4,
		ModelId:            "opencsg/wukong",
		DatasetId:          "opencsg/hellaswag",
		Epochs:             3,
		ShareMode:          false,
	}

	ctx := context.TODO()

	cfg := &config.Config{}
	cfg.Argo.QuotaGPUNumber = "1"

	mockDeployer := mockdeploy.NewMockDeployer(t)
	mockUser := mockdb.NewMockUserStore(t)
	modelStore := mockdb.NewMockModelStore(t)
	spaceResStore := mockdb.NewMockSpaceResourceStore(t)
	datasetStore := mockdb.NewMockDatasetStore(t)
	mirrorStore := mockdb.NewMockMirrorStore(t)
	tokenStore := mockdb.NewMockAccessTokenStore(t)
	repoStore := mockdb.NewMockRepoStore(t)
	frameStore := mockdb.NewMockRuntimeFrameworksStore(t)
	argoStore := mockdb.NewMockArgoWorkFlowStore(t)
	acctComp := mockComps.NewMockAccountingComponent(t)
	repoComp := mockComps.NewMockRepoComponent(t)

	req2 := types.FinetuneReq{
		UserUUID:           "testuser",
		TaskName:           "testtask",
		ModelId:            "opencsg/wukong",
		Username:           "testuser",
		ResourceId:         4,
		DatasetId:          "opencsg/hellaswag",
		RuntimeFrameworkId: 1,
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
		Image:    "lm-finetune:0.4.6",
		RepoType: "model",
		TaskType: "finetune",
		Token:    "foo",
		Epochs:   3,
	}

	resource, err := json.Marshal(req2.Hardware)
	require.Nil(t, err)

	c := NewTestFinetuneComponent(cfg, mockDeployer, mockUser, modelStore, spaceResStore, datasetStore, mirrorStore,
		tokenStore, repoStore, frameStore, argoStore, acctComp, repoComp)

	mockUser.EXPECT().FindByUsername(ctx, req.Username).Return(database.User{
		Username: req.Username,
		UUID:     req.Username,
		ID:       1,
	}, nil).Once()

	tokenStore.EXPECT().FindByUID(ctx, int64(1)).Return(&database.AccessToken{Token: "foo"}, nil)

	frameStore.EXPECT().FindEnabledByID(ctx, int64(1)).Return(&database.RuntimeFramework{
		ID:          1,
		FrameImage:  "lm-finetune:0.4.6",
		ComputeType: string(types.ResourceTypeGPU),
	}, nil)

	spaceResStore.EXPECT().FindByID(ctx, int64(4)).Return(&database.SpaceResource{
		ID:        4,
		Resources: string(resource),
	}, nil)

	repoComp.EXPECT().CheckAccountAndResource(ctx, req.Username, "", int64(0), mock.Anything).Return(nil)

	mockDeployer.EXPECT().SubmitFinetuneJob(ctx, req2).Return(&types.ArgoWorkFlowRes{
		ID:       1,
		TaskName: "test",
	}, nil)

	e, err := c.CreateFinetuneJob(ctx, req)
	require.NotNil(t, e)
	require.Equal(t, "test", e.TaskName)
	require.Nil(t, err)
}

func TestEvaluationComponent_GetFinetune(t *testing.T) {
	ctx := context.TODO()

	cfg := &config.Config{}
	cfg.Argo.QuotaGPUNumber = "1"

	mockDeployer := mockdeploy.NewMockDeployer(t)
	mockUser := mockdb.NewMockUserStore(t)
	modelStore := mockdb.NewMockModelStore(t)
	spaceResStore := mockdb.NewMockSpaceResourceStore(t)
	datasetStore := mockdb.NewMockDatasetStore(t)
	mirrorStore := mockdb.NewMockMirrorStore(t)
	tokenStore := mockdb.NewMockAccessTokenStore(t)
	repoStore := mockdb.NewMockRepoStore(t)
	frameStore := mockdb.NewMockRuntimeFrameworksStore(t)
	argoStore := mockdb.NewMockArgoWorkFlowStore(t)
	acctComp := mockComps.NewMockAccountingComponent(t)
	repoComp := mockComps.NewMockRepoComponent(t)

	req := types.FinetineGetReq{
		Username: "testuser",
		ID:       1,
	}

	c := NewTestFinetuneComponent(cfg, mockDeployer, mockUser, modelStore, spaceResStore, datasetStore, mirrorStore,
		tokenStore, repoStore, frameStore, argoStore, acctComp, repoComp)

	argoStore.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
		ID:       1,
		TaskName: "test",
	}, nil)

	e, err := c.GetFinetuneJob(ctx, req)

	require.NotNil(t, e)
	require.Equal(t, "test", e.TaskName)
	require.Nil(t, err)
}

func TestEvaluationComponent_DeleteFinetune(t *testing.T) {
	ctx := context.TODO()

	cfg := &config.Config{}
	cfg.Argo.QuotaGPUNumber = "1"

	mockDeployer := mockdeploy.NewMockDeployer(t)
	mockUser := mockdb.NewMockUserStore(t)
	modelStore := mockdb.NewMockModelStore(t)
	spaceResStore := mockdb.NewMockSpaceResourceStore(t)
	datasetStore := mockdb.NewMockDatasetStore(t)
	mirrorStore := mockdb.NewMockMirrorStore(t)
	tokenStore := mockdb.NewMockAccessTokenStore(t)
	repoStore := mockdb.NewMockRepoStore(t)
	frameStore := mockdb.NewMockRuntimeFrameworksStore(t)
	argoStore := mockdb.NewMockArgoWorkFlowStore(t)
	acctComp := mockComps.NewMockAccountingComponent(t)
	repoComp := mockComps.NewMockRepoComponent(t)

	c := NewTestFinetuneComponent(cfg, mockDeployer, mockUser, modelStore, spaceResStore, datasetStore, mirrorStore,
		tokenStore, repoStore, frameStore, argoStore, acctComp, repoComp)

	argoStore.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
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
	argoStore.EXPECT().DeleteWorkFlow(ctx, int64(1)).Return(nil)

	mockDeployer.EXPECT().DeleteFinetuneJob(ctx, req).Return(nil)

	err := c.DeleteFinetuneJob(ctx, req)
	require.Nil(t, err)
}

func TestFinetuneComponent_ReadJobLogsNonStream(t *testing.T) {
	ctx := context.TODO()
	cfg := &config.Config{}

	t.Run("find-workflow-logs-success", func(t *testing.T) {
		mockDeployer := mockdeploy.NewMockDeployer(t)
		argoStore := mockdb.NewMockArgoWorkFlowStore(t)

		c := NewTestFinetuneComponent(cfg, mockDeployer, nil, nil, nil, nil, nil,
			nil, nil, nil, argoStore, nil, nil)

		req := types.FinetuneLogReq{
			ID: 1,
		}

		argoStore.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
			ID:         1,
			TaskId:     "test-task",
			SubmitTime: time.Now(),
		}, nil)

		expectedLogs := &loki.LokiQueryResponse{}
		mockDeployer.EXPECT().GetWorkflowLogsNonStream(ctx, mock.Anything).Return(expectedLogs, nil)

		logs, err := c.ReadJobLogsNonStream(ctx, req)
		require.NoError(t, err)
		require.Equal(t, "", logs)
	})

	t.Run("find-workflow-logs-failed", func(t *testing.T) {
		mockDeployer := mockdeploy.NewMockDeployer(t)
		argoStore := mockdb.NewMockArgoWorkFlowStore(t)

		c := NewTestFinetuneComponent(cfg, mockDeployer, nil, nil, nil, nil, nil,
			nil, nil, nil, argoStore, nil, nil)

		req := types.FinetuneLogReq{
			ID: 1,
		}

		expectedErr := errors.New("not found")
		argoStore.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{}, expectedErr)

		_, err := c.ReadJobLogsNonStream(ctx, req)
		require.NotNil(t, err)
	})

	t.Run("get-logs-failed", func(t *testing.T) {
		mockDeployer := mockdeploy.NewMockDeployer(t)
		argoStore := mockdb.NewMockArgoWorkFlowStore(t)

		c := NewTestFinetuneComponent(cfg, mockDeployer, nil, nil, nil, nil, nil,
			nil, nil, nil, argoStore, nil, nil)
		req := types.FinetuneLogReq{
			ID: 1,
		}

		argoStore.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
			ID:         1,
			TaskId:     "test-task",
			SubmitTime: time.Now(),
		}, nil)

		expectedErr := errors.New("failed to get logs")
		mockDeployer.EXPECT().GetWorkflowLogsNonStream(ctx, mock.Anything).Return(nil, expectedErr)

		_, err := c.ReadJobLogsNonStream(ctx, req)
		require.NotNil(t, err)
	})
}

func TestFinetuneComponent_ReadJobLogsInStream(t *testing.T) {
	ctx := context.TODO()
	cfg := &config.Config{}

	t.Run("wf-logs-success", func(t *testing.T) {
		mockDeployer := mockdeploy.NewMockDeployer(t)
		argoStore := mockdb.NewMockArgoWorkFlowStore(t)

		c := NewTestFinetuneComponent(cfg, mockDeployer, nil, nil, nil, nil, nil,
			nil, nil, nil, argoStore, nil, nil)

		req := types.FinetuneLogReq{
			ID: 1,
		}

		argoStore.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
			ID:         1,
			TaskId:     "test-task",
			SubmitTime: time.Now(),
		}, nil)

		expectedReader := &deploy.MultiLogReader{}
		mockDeployer.EXPECT().GetWorkflowLogsInStream(ctx, mock.Anything).Return(expectedReader, nil)

		reader, err := c.ReadJobLogsInStream(ctx, req)
		require.NoError(t, err)
		require.Equal(t, expectedReader, reader)
	})

	t.Run("find workflow failed", func(t *testing.T) {
		mockDeployer := mockdeploy.NewMockDeployer(t)
		argoStore := mockdb.NewMockArgoWorkFlowStore(t)

		c := NewTestFinetuneComponent(cfg, mockDeployer, nil, nil, nil, nil, nil,
			nil, nil, nil, argoStore, nil, nil)

		req := types.FinetuneLogReq{
			ID: 1,
		}

		expectedErr := errors.New("not found")
		argoStore.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{}, expectedErr)

		_, err := c.ReadJobLogsInStream(ctx, req)
		require.NotNil(t, err)
	})
}
