package component

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdeploy "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockComps "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func newTestEvaluationComponentForClaw(t *testing.T) (*evaluationComponentImpl, *mockdeploy.MockDeployer, *mockComps.MockRepoComponent, *mockdb.MockArgoWorkFlowStore) {
	t.Helper()
	mockDeployer := mockdeploy.NewMockDeployer(t)
	mockUser := mockdb.NewMockUserStore(t)
	mockSpaceRes := mockdb.NewMockSpaceResourceStore(t)
	mockFrame := mockdb.NewMockRuntimeFrameworksStore(t)
	mockWorkflow := mockdb.NewMockArgoWorkFlowStore(t)
	mockCluster := mockdb.NewMockClusterInfoStore(t)
	mockToken := mockdb.NewMockAccessTokenStore(t)
	mockRepo := mockComps.NewMockRepoComponent(t)
	mockUserSvc := mockrpc.NewMockUserSvcClient(t)

	c := &evaluationComponentImpl{
		deployer:              mockDeployer,
		userStore:             mockUser,
		spaceResourceStore:    mockSpaceRes,
		runtimeFrameworkStore: mockFrame,
		workflowStore:         mockWorkflow,
		clusterStore:          mockCluster,
		tokenStore:            mockToken,
		repoComponent:         mockRepo,
		userSvcClient:         mockUserSvc,
		config:                &config.Config{},
	}
	c.config.AIGateway.PublicAIGatewayURL = "http://aigateway.test/v1"
	return c, mockDeployer, mockRepo, mockWorkflow
}

func TestEvaluationComponent_CreateClawEvaluation(t *testing.T) {
	ctx := context.Background()
	req := types.EvaluationReq{
		Username:           "user1",
		OwnerNamespace:     "user1",
		TaskName:           "claw-job",
		RuntimeFrameworkId: 1,
		ResourceId:         2,
		Model:              "glm-5.1",
		BaseURL:            "http://localhost:11435/v1",
		ApiKey:             "sk-test",
		Tasks:              "1-9",
		Trials:             1,
		Parallel:           2,
	}

	c, mockDeployer, mockRepo, _ := newTestEvaluationComponentForClaw(t)
	mockUser := c.userStore.(*mockdb.MockUserStore)
	mockSpaceRes := c.spaceResourceStore.(*mockdb.MockSpaceResourceStore)
	mockFrame := c.runtimeFrameworkStore.(*mockdb.MockRuntimeFrameworksStore)
	mockCluster := c.clusterStore.(*mockdb.MockClusterInfoStore)
	mockToken := c.tokenStore.(*mockdb.MockAccessTokenStore)

	mockUser.EXPECT().FindByUsername(ctx, "user1").Return(database.User{Username: "user1", UUID: "uuid1", RoleMask: "admin"}, nil)
	mockFrame.EXPECT().FindEnabledByID(ctx, int64(1)).Return(&database.RuntimeFramework{
		ID:          1,
		FrameName:   types.ClawEvalFrameName,
		FrameImage:  "opencsghq/claw-eval:1.0.0",
		ComputeType: "cpu",
	}, nil)
	mockSpaceRes.EXPECT().FindByID(ctx, int64(2)).Return(&database.SpaceResource{
		ClusterID: "cluster1",
		Name:      "cpu-small",
		Resources: `{"cpu":{"num":"4"},"memory":"8Gi"}`,
	}, nil)
	mockRepo.EXPECT().CheckAccountAndResource(ctx, mock.Anything, mock.Anything).Return(&types.CheckExclusiveResp{}, nil)
	mockCluster.EXPECT().FindNodeByClusterID(ctx, "cluster1").Return([]database.ClusterNode{}, nil)
	mockToken.EXPECT().FindBuiltinByNsUUID(ctx, "uuid1", string(types.AccessTokenAppAIGateway)).Return(&database.AccessToken{
		Token: "gk-builtin-key",
	}, nil)
	mockDeployer.EXPECT().SubmitClawEvaluation(ctx, mock.MatchedBy(func(r types.ClawEvaluationReq) bool {
		return r.TaskType == types.TaskTypeClawEval &&
			r.Model == "glm-5.1" &&
			r.Image == "opencsghq/claw-eval:1.0.0" &&
			r.Tasks == "1-9" &&
			r.ApiKey == "sk-test" &&
			r.JudgeBaseURL == "http://aigateway.test/v1" &&
			r.JudgeApiKey == "gk-builtin-key"
	})).Return(&types.ArgoWorkFlowRes{ID: 1}, nil)

	resp, err := c.CreateEvaluation(ctx, req)
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.ID)
}

func TestEvaluationComponent_CreateClawEvaluation_AutoBuiltinJudgeAPIKey(t *testing.T) {
	ctx := context.Background()
	req := types.EvaluationReq{
		Username:           "user1",
		OwnerNamespace:     "user1",
		TaskName:           "claw-job",
		RuntimeFrameworkId: 1,
		ResourceId:         2,
		ModelId:            "glm-5.1",
		BaseURL:            "http://localhost:11435/v1",
		Tasks:              "T001",
	}

	c, mockDeployer, mockRepo, _ := newTestEvaluationComponentForClaw(t)
	mockUser := c.userStore.(*mockdb.MockUserStore)
	mockSpaceRes := c.spaceResourceStore.(*mockdb.MockSpaceResourceStore)
	mockFrame := c.runtimeFrameworkStore.(*mockdb.MockRuntimeFrameworksStore)
	mockCluster := c.clusterStore.(*mockdb.MockClusterInfoStore)
	mockToken := c.tokenStore.(*mockdb.MockAccessTokenStore)

	mockUser.EXPECT().FindByUsername(ctx, "user1").Return(database.User{Username: "user1", UUID: "uuid1", RoleMask: "admin"}, nil)
	mockFrame.EXPECT().FindEnabledByID(ctx, int64(1)).Return(&database.RuntimeFramework{
		FrameName:   types.ClawEvalFrameName,
		FrameImage:  "opencsghq/claw-eval:1.0.0",
		ComputeType: "cpu",
	}, nil)
	mockSpaceRes.EXPECT().FindByID(ctx, int64(2)).Return(&database.SpaceResource{
		ClusterID: "cluster1",
		Name:      "cpu-small",
		Resources: `{"cpu":{"num":"4"},"memory":"8Gi"}`,
	}, nil)
	mockRepo.EXPECT().CheckAccountAndResource(ctx, mock.Anything, mock.Anything).Return(&types.CheckExclusiveResp{}, nil)
	mockCluster.EXPECT().FindNodeByClusterID(ctx, "cluster1").Return([]database.ClusterNode{}, nil)
	mockToken.EXPECT().FindBuiltinByNsUUID(ctx, "uuid1", string(types.AccessTokenAppAIGateway)).Return(&database.AccessToken{
		Token: "gk-builtin-key",
	}, nil)
	mockDeployer.EXPECT().SubmitClawEvaluation(ctx, mock.MatchedBy(func(r types.ClawEvaluationReq) bool {
		return r.ApiKey == "gk-builtin-key" &&
			r.Model == "glm-5.1" &&
			r.JudgeApiKey == "gk-builtin-key" &&
			r.JudgeBaseURL == "http://aigateway.test/v1"
	})).Return(&types.ArgoWorkFlowRes{ID: 2}, nil)

	resp, err := c.CreateEvaluation(ctx, req)
	require.NoError(t, err)
	require.Equal(t, int64(2), resp.ID)
}

func TestEvaluationComponent_CreateClawEvaluation_AutoJudgeAPIKeyFallback(t *testing.T) {
	ctx := context.Background()
	req := types.EvaluationReq{
		Username:           "user1",
		OwnerNamespace:     "user1",
		TaskName:           "claw-job",
		RuntimeFrameworkId: 1,
		ResourceId:         2,
		Model:              "glm-5.1",
		BaseURL:            "http://localhost:11435/v1",
		Tasks:              "T001",
	}

	c, mockDeployer, mockRepo, _ := newTestEvaluationComponentForClaw(t)
	mockUser := c.userStore.(*mockdb.MockUserStore)
	mockSpaceRes := c.spaceResourceStore.(*mockdb.MockSpaceResourceStore)
	mockFrame := c.runtimeFrameworkStore.(*mockdb.MockRuntimeFrameworksStore)
	mockCluster := c.clusterStore.(*mockdb.MockClusterInfoStore)
	mockToken := c.tokenStore.(*mockdb.MockAccessTokenStore)
	mockUserSvc := c.userSvcClient.(*mockrpc.MockUserSvcClient)

	mockUser.EXPECT().FindByUsername(ctx, "user1").Return(database.User{Username: "user1", UUID: "uuid1", RoleMask: "admin"}, nil)
	mockFrame.EXPECT().FindEnabledByID(ctx, int64(1)).Return(&database.RuntimeFramework{
		FrameName:   types.ClawEvalFrameName,
		FrameImage:  "opencsghq/claw-eval:1.0.0",
		ComputeType: "cpu",
	}, nil)
	mockSpaceRes.EXPECT().FindByID(ctx, int64(2)).Return(&database.SpaceResource{
		ClusterID: "cluster1",
		Name:      "cpu-small",
		Resources: `{"cpu":{"num":"4"},"memory":"8Gi"}`,
	}, nil)
	mockRepo.EXPECT().CheckAccountAndResource(ctx, mock.Anything, mock.Anything).Return(&types.CheckExclusiveResp{}, nil)
	mockCluster.EXPECT().FindNodeByClusterID(ctx, "cluster1").Return([]database.ClusterNode{}, nil)
	mockToken.EXPECT().FindBuiltinByNsUUID(ctx, "uuid1", string(types.AccessTokenAppAIGateway)).Return(nil, errorx.ErrNotFound)
	mockUserSvc.EXPECT().GetOrCreateFirstAvaiTokens(
		ctx, "user1", "user1", string(types.AccessTokenAppAIGateway), "claw-eval",
	).Return("gk-fallback-key", nil)
	mockDeployer.EXPECT().SubmitClawEvaluation(ctx, mock.MatchedBy(func(r types.ClawEvaluationReq) bool {
		return r.ApiKey == "gk-fallback-key" &&
			r.JudgeApiKey == "gk-fallback-key" &&
			r.JudgeBaseURL == "http://aigateway.test/v1"
	})).Return(&types.ArgoWorkFlowRes{ID: 3}, nil)

	resp, err := c.CreateEvaluation(ctx, req)
	require.NoError(t, err)
	require.Equal(t, int64(3), resp.ID)
}

func TestEvaluationComponent_CheckEvaluationLogPermission(t *testing.T) {
	ctx := context.Background()
	c, _, _, mockWorkflow := newTestEvaluationComponentForClaw(t)
	mockUserSvc := c.userSvcClient.(*mockrpc.MockUserSvcClient)

	mockWorkflow.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
		ID:       1,
		TaskType: types.TaskTypeClawEval,
		UserUUID: "uuid1",
	}, nil)
	mockUserSvc.EXPECT().GetUserByName(ctx, "user1").Return(&types.User{
		UUID:     "uuid1",
		Username: "user1",
	}, nil)
	mockUserSvc.EXPECT().GetNameSpaceInfoByUUID(ctx, "uuid1").Return(&rpc.Namespace{
		UUID:   "uuid1",
		NSType: "user",
	}, nil)

	allow, wf, err := c.checkEvaluationLogPermission(ctx, types.EvaluationLogReq{CurrentUser: "user1", ID: 1})
	require.NoError(t, err)
	require.True(t, allow)
	require.Equal(t, int64(1), wf.ID)

	mockWorkflow.EXPECT().FindByID(ctx, int64(2)).Return(database.ArgoWorkflow{
		ID:       2,
		TaskType: types.TaskTypeFinetune,
		UserUUID: "uuid1",
	}, nil)
	_, _, err = c.checkEvaluationLogPermission(ctx, types.EvaluationLogReq{CurrentUser: "user1", ID: 2})
	require.ErrorIs(t, err, errorx.ErrForbidden)
}

func TestEvaluationComponent_CreateClawEvaluation_InvalidResourceJSON(t *testing.T) {
	ctx := context.Background()
	req := types.EvaluationReq{
		Username:           "user1",
		TaskName:           "claw-job",
		RuntimeFrameworkId: 1,
		ResourceId:         2,
		Model:              "glm-5.1",
		BaseURL:            "http://localhost:11435/v1",
		ApiKey:             "sk-test",
	}

	c, _, _, _ := newTestEvaluationComponentForClaw(t)
	mockUser := c.userStore.(*mockdb.MockUserStore)
	mockFrame := c.runtimeFrameworkStore.(*mockdb.MockRuntimeFrameworksStore)
	mockSpaceRes := c.spaceResourceStore.(*mockdb.MockSpaceResourceStore)

	mockUser.EXPECT().FindByUsername(ctx, "user1").Return(database.User{Username: "user1", RoleMask: "admin"}, nil)
	mockFrame.EXPECT().FindEnabledByID(ctx, int64(1)).Return(&database.RuntimeFramework{
		FrameName:   types.ClawEvalFrameName,
		ComputeType: "cpu",
	}, nil)
	mockSpaceRes.EXPECT().FindByID(ctx, int64(2)).Return(&database.SpaceResource{
		Resources: `{invalid`,
	}, nil)

	_, err := c.CreateEvaluation(ctx, req)
	require.Error(t, err)
}

func TestEvaluationComponent_CreateClawEvaluation_Forbidden(t *testing.T) {
	ctx := context.Background()
	req := types.EvaluationReq{
		Username:           "user1",
		OwnerNamespace:     "org1",
		TaskName:           "claw-job",
		RuntimeFrameworkId: 1,
		ResourceId:         2,
		Model:              "glm-5.1",
		BaseURL:            "http://localhost:11435/v1",
		ApiKey:             "sk-test",
	}

	c, _, mockRepo, _ := newTestEvaluationComponentForClaw(t)
	mockUser := c.userStore.(*mockdb.MockUserStore)

	mockUser.EXPECT().FindByUsername(ctx, "user1").Return(database.User{Username: "user1"}, nil)
	mockRepo.EXPECT().CheckCurrentUserPermission(ctx, "user1", "org1", membership.RoleWrite).Return(false, nil)

	_, err := c.CreateEvaluation(ctx, req)
	require.ErrorIs(t, err, errorx.ErrForbidden)
}

func TestEvaluationComponent_GetClawEvaluation(t *testing.T) {
	ctx := context.Background()
	c, _, mockRepo, mockWorkflow := newTestEvaluationComponentForClaw(t)

	mockWorkflow.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
		ID:       1,
		RepoIds:  []string{"glm-5.1"},
		TaskType: types.TaskTypeClawEval,
		Username: "user1",
		TaskName: "claw-job",
		TaskId:   "task-1",
		Status:   "Succeeded",
	}, nil)

	res, err := c.GetEvaluation(ctx, types.EvaluationGetReq{ID: 1, Username: "user1"})
	require.NoError(t, err)
	require.Equal(t, types.TaskTypeClawEval, res.TaskType)
	require.Empty(t, res.Datasets)

	mockWorkflow.EXPECT().FindByID(ctx, int64(2)).Return(database.ArgoWorkflow{
		ID:       2,
		Username: "user1",
		TaskType: types.TaskTypeEvaluation,
	}, nil)
	mockRepo.EXPECT().CheckCurrentUserPermission(ctx, "other", "user1", membership.RoleRead).Return(false, nil)
	_, err = c.GetEvaluation(ctx, types.EvaluationGetReq{ID: 2, Username: "other"})
	require.ErrorIs(t, err, errorx.ErrForbidden)
}

func TestEvaluationComponent_GetClawEvaluationSummary(t *testing.T) {
	summaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tasks": 129,
			"trials_per_task": 3,
			"errored": 3,
			"avg_score": 0.72,
			"pass_hat_3": 40,
			"pass_at_3": 55
		}`))
	}))
	defer summaryServer.Close()

	ctx := context.Background()
	c, _, _, mockWorkflow := newTestEvaluationComponentForClaw(t)

	mockWorkflow.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
		ID:        1,
		RepoIds:   []string{"glm-5.1"},
		TaskType:  types.TaskTypeClawEval,
		Username:  "user1",
		TaskName:  "claw-job",
		TaskId:    "task-1",
		Status:    "Succeeded",
		ResultURL: summaryServer.URL,
	}, nil)

	res, err := c.GetEvaluation(ctx, types.EvaluationGetReq{ID: 1, Username: "user1"})
	require.NoError(t, err)
	require.NotNil(t, res.Summary)
	require.Equal(t, 129, res.Summary.Tasks)
	require.Equal(t, 3, res.Summary.TrialsPerTask)
	require.Equal(t, 3, res.Summary.Errored)
	require.InDelta(t, 0.72, res.Summary.AvgScore, 0.0001)
	require.Equal(t, 40, res.Summary.PassHatK)
	require.Equal(t, 55, res.Summary.PassAtK)
}

func TestEvaluationComponent_DeleteClawEvaluation(t *testing.T) {
	ctx := context.Background()
	c, mockDeployer, _, mockWorkflow := newTestEvaluationComponentForClaw(t)

	mockWorkflow.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
		ID:        1,
		TaskType:  types.TaskTypeClawEval,
		Username:  "user1",
		TaskId:    "task-1",
		Namespace: "ns1",
		ClusterID: "cluster1",
	}, nil)
	mockWorkflow.EXPECT().DeleteWorkFlow(ctx, int64(1)).Return(nil)
	mockDeployer.EXPECT().DeleteEvaluation(ctx, types.EvaluationDelReq{
		ID:        1,
		Username:  "user1",
		TaskID:    "task-1",
		Namespace: "ns1",
		ClusterID: "cluster1",
	}).Return(nil)

	err := c.DeleteEvaluation(ctx, types.EvaluationDelReq{ID: 1, Username: "user1"})
	require.NoError(t, err)
}

func TestEvaluationComponent_CreateClawEvaluation_MissingFields(t *testing.T) {
	ctx := context.Background()
	c, _, _, _ := newTestEvaluationComponentForClaw(t)
	mockUser := c.userStore.(*mockdb.MockUserStore)
	mockFrame := c.runtimeFrameworkStore.(*mockdb.MockRuntimeFrameworksStore)

	mockUser.EXPECT().FindByUsername(ctx, "user1").Return(database.User{Username: "user1", RoleMask: "admin"}, nil)
	mockFrame.EXPECT().FindEnabledByID(ctx, int64(1)).Return(&database.RuntimeFramework{
		FrameName: types.ClawEvalFrameName,
	}, nil)

	_, err := c.CreateEvaluation(ctx, types.EvaluationReq{
		Username:           "user1",
		RuntimeFrameworkId: 1,
		ResourceId:         2,
		BaseURL:            "http://localhost/v1",
		ApiKey:             "sk-test",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "task_name is required")

	_, err = c.CreateEvaluation(ctx, types.EvaluationReq{
		Username:           "user1",
		TaskName:           "claw-job",
		RuntimeFrameworkId: 1,
		Model:              "glm-5.1",
		BaseURL:            "http://localhost/v1",
		ApiKey:             "sk-test",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "resource_id is required")
}

func TestEvaluationComponent_ReadJobLogsNonStream(t *testing.T) {
	ctx := context.Background()
	c, mockDeployer, _, mockWorkflow := newTestEvaluationComponentForClaw(t)
	mockUserSvc := c.userSvcClient.(*mockrpc.MockUserSvcClient)
	c.config.LogCollector.LineSeparator = "\n"

	mockUserSvc.EXPECT().GetUserByName(ctx, "user1").Return(&types.User{
		UUID:     "uuid1",
		Username: "user1",
	}, nil)
	mockWorkflow.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
		ID:       1,
		TaskType: types.TaskTypeClawEval,
		TaskId:   "task-1",
		UserUUID: "uuid1",
	}, nil)
	mockUserSvc.EXPECT().GetNameSpaceInfoByUUID(ctx, "uuid1").Return(&rpc.Namespace{
		UUID:   "uuid1",
		NSType: "user",
	}, nil)
	mockDeployer.EXPECT().GetWorkflowLogsNonStream(ctx, mock.Anything, mock.Anything).Return(&loki.LokiQueryResponse{}, nil)

	logs, err := c.ReadJobLogsNonStream(ctx, types.EvaluationLogReq{CurrentUser: "user1", ID: 1})
	require.NoError(t, err)
	require.Equal(t, "", logs)
}

func TestEvaluationComponent_ReadJobLogsInStream(t *testing.T) {
	ctx := context.Background()
	c, mockDeployer, _, mockWorkflow := newTestEvaluationComponentForClaw(t)
	mockUserSvc := c.userSvcClient.(*mockrpc.MockUserSvcClient)

	mockUserSvc.EXPECT().GetUserByName(ctx, "user1").Return(&types.User{
		UUID:     "uuid1",
		Username: "user1",
	}, nil)
	mockWorkflow.EXPECT().FindByID(ctx, int64(1)).Return(database.ArgoWorkflow{
		ID:       1,
		TaskType: types.TaskTypeClawEval,
		TaskId:   "task-1",
		UserUUID: "uuid1",
	}, nil)
	mockUserSvc.EXPECT().GetNameSpaceInfoByUUID(ctx, "uuid1").Return(&rpc.Namespace{
		UUID:   "uuid1",
		NSType: "user",
	}, nil)
	mockDeployer.EXPECT().GetWorkflowLogsInStream(ctx, mock.Anything, mock.Anything).Return(&deploy.MultiLogReader{}, nil)

	reader, err := c.ReadJobLogsInStream(ctx, types.EvaluationLogReq{CurrentUser: "user1", ID: 1})
	require.NoError(t, err)
	require.NotNil(t, reader)
}
