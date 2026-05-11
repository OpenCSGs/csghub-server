package component

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/bwmarrin/snowflake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdeploy "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type testPlatformDataflowComponent struct {
	*platformDataflowComponentImpl
	mocks struct {
		deployer           *mockdeploy.MockDeployer
		workflowStore      *mockdb.MockArgoWorkFlowStore
		userSvcClient      *mockrpc.MockUserSvcClient
		clusterStore       *mockdb.MockClusterInfoStore
		spaceResourceStore *mockdb.MockSpaceResourceStore
		repoComponent      *mockcomp.MockRepoComponent
	}
}

func newTestPlatformDataflowComponent(t *testing.T) *testPlatformDataflowComponent {
	node, err := snowflake.NewNode(1)
	require.NoError(t, err)

	c := &testPlatformDataflowComponent{
		platformDataflowComponentImpl: &platformDataflowComponentImpl{
			snowflakeNode: node,
			config:        &config.Config{},
		},
	}

	c.mocks.deployer = mockdeploy.NewMockDeployer(t)
	c.deployer = c.mocks.deployer

	c.mocks.workflowStore = mockdb.NewMockArgoWorkFlowStore(t)
	c.workflowStore = c.mocks.workflowStore

	c.mocks.userSvcClient = mockrpc.NewMockUserSvcClient(t)
	c.userSvcClient = c.mocks.userSvcClient

	c.mocks.clusterStore = mockdb.NewMockClusterInfoStore(t)
	c.clusterStore = c.mocks.clusterStore

	c.mocks.spaceResourceStore = mockdb.NewMockSpaceResourceStore(t)
	c.spaceResourceStore = c.mocks.spaceResourceStore

	c.mocks.repoComponent = mockcomp.NewMockRepoComponent(t)
	c.repoComponent = c.mocks.repoComponent

	return c
}

func TestPlatformDataflowComponent_CreateJob(t *testing.T) {
	ctx := context.TODO()
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			RepoIds:    []string{"repo1", "repo2"},
			ResourceId: 100,
			JobName:    "test-job",
			JobDesc:    "test description",
			Template: types.ArgoFlowTemplate{
				Image: "test-image:latest",
			},
		}

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "testuser",
			UUID:   "user-uuid-1",
			NSType: string(database.UserNamespace),
		}

		resource := &database.SpaceResource{
			ID:        100,
			Name:      "test-resource",
			ClusterID: "cluster-1",
			Resources: `{"cpu": {"num": "2"}, "memory": "4Gi"}`,
		}

		clusterNodes := []database.ClusterNode{
			{
				Name: "node-1",
			},
		}

		createdWF := &database.ArgoWorkflow{
			ID:         1,
			TaskId:     "df-abc123",
			TaskName:   req.JobName,
			UserUUID:   req.NSUUID,
			Username:   req.Username,
			Status:     v1alpha1.WorkflowPending,
			SubmitTime: now,
		}

		deployResp := &types.DataflowArgoJobResp{
			ArgoTaskID: "df-abc123",
			JobID:      "df-abc123",
			JobName:    req.JobName,
			Status:     "Pending",
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)
		c.mocks.userSvcClient.EXPECT().GetOrCreateFirstAvaiTokens(ctx, req.Username, req.Username, string(types.AccessTokenAppGit), "dataflow").Return("test-token", nil)
		c.mocks.spaceResourceStore.EXPECT().FindByID(ctx, req.ResourceId).Return(resource, nil)
		c.mocks.repoComponent.EXPECT().CheckAccountAndResource(ctx, ns.Path, resource.ClusterID, int64(0), resource).Return(&types.CheckExclusiveResp{}, nil)
		c.mocks.clusterStore.EXPECT().FindNodeByClusterID(ctx, resource.ClusterID).Return(clusterNodes, nil)
		c.mocks.workflowStore.EXPECT().CreateWorkFlow(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			return wf.TaskName == req.JobName &&
				wf.UserUUID == req.NSUUID &&
				wf.Username == req.Username
		})).Return(createdWF, nil)
		c.mocks.deployer.EXPECT().CreateDataflowJob(ctx, mock.MatchedBy(func(r *types.DataflowArgoJobReq) bool {
			return r.ID == createdWF.ID && r.ArgoTaskID == createdWF.TaskId && r.AccessToken == "test-token"
		})).Return(deployResp, nil)

		resp, err := c.CreateJob(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, createdWF.ID, resp.ID)
		require.Equal(t, createdWF.TaskId, resp.ArgoTaskID)
	})

	t.Run("user not found", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username: "nonexistent",
			NSUUID:   "user-uuid-1",
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(nil, errors.New("user not found"))

		resp, err := c.CreateJob(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "current user not found")
	})

	t.Run("namespace not found", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username: "testuser",
			NSUUID:   "nonexistent-uuid",
		}

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(nil, errors.New("namespace not found"))

		resp, err := c.CreateJob(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "target namespace not found")
	})

	t.Run("resource not found", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ResourceId: 999,
		}

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "testuser",
			UUID:   "user-uuid-1",
			NSType: string(database.UserNamespace),
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)
		c.mocks.userSvcClient.EXPECT().GetOrCreateFirstAvaiTokens(ctx, req.Username, req.Username, string(types.AccessTokenAppGit), "dataflow").Return("test-token", nil)
		c.mocks.spaceResourceStore.EXPECT().FindByID(ctx, req.ResourceId).Return(nil, errors.New("resource not found"))

		resp, err := c.CreateJob(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "cannot find resource")
	})

	t.Run("check account and resource failed", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ResourceId: 100,
		}

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "testuser",
			UUID:   "user-uuid-1",
			NSType: string(database.UserNamespace),
		}

		resource := &database.SpaceResource{
			ID:        100,
			ClusterID: "cluster-1",
			Resources: `{"cpu": {"num": "2"}}`,
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)
		c.mocks.userSvcClient.EXPECT().GetOrCreateFirstAvaiTokens(ctx, req.Username, req.Username, string(types.AccessTokenAppGit), "dataflow").Return("test-token", nil)
		c.mocks.spaceResourceStore.EXPECT().FindByID(ctx, req.ResourceId).Return(resource, nil)
		c.mocks.repoComponent.EXPECT().CheckAccountAndResource(ctx, ns.Path, resource.ClusterID, int64(0), resource).Return(nil, errors.New("resource unavailable"))

		resp, err := c.CreateJob(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "failed to check account and resource")
	})

	t.Run("cluster nodes not found", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ResourceId: 100,
		}

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "testuser",
			UUID:   "user-uuid-1",
			NSType: string(database.UserNamespace),
		}

		resource := &database.SpaceResource{
			ID:        100,
			ClusterID: "cluster-1",
			Resources: `{"cpu": {"num": "2"}}`,
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)
		c.mocks.userSvcClient.EXPECT().GetOrCreateFirstAvaiTokens(ctx, req.Username, req.Username, string(types.AccessTokenAppGit), "dataflow").Return("test-token", nil)
		c.mocks.spaceResourceStore.EXPECT().FindByID(ctx, req.ResourceId).Return(resource, nil)
		c.mocks.repoComponent.EXPECT().CheckAccountAndResource(ctx, ns.Path, resource.ClusterID, int64(0), resource).Return(&types.CheckExclusiveResp{}, nil)
		c.mocks.clusterStore.EXPECT().FindNodeByClusterID(ctx, resource.ClusterID).Return(nil, errors.New("cluster not found"))

		resp, err := c.CreateJob(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "failed to find nodes by clusterID")
	})

	t.Run("create workflow failed", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ResourceId: 100,
			JobName:    "test-job",
		}

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "testuser",
			UUID:   "user-uuid-1",
			NSType: string(database.UserNamespace),
		}

		resource := &database.SpaceResource{
			ID:        100,
			ClusterID: "cluster-1",
			Resources: `{"cpu": {"num": "2"}}`,
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)
		c.mocks.userSvcClient.EXPECT().GetOrCreateFirstAvaiTokens(ctx, req.Username, req.Username, string(types.AccessTokenAppGit), "dataflow").Return("test-token", nil)
		c.mocks.spaceResourceStore.EXPECT().FindByID(ctx, req.ResourceId).Return(resource, nil)
		c.mocks.repoComponent.EXPECT().CheckAccountAndResource(ctx, ns.Path, resource.ClusterID, int64(0), resource).Return(&types.CheckExclusiveResp{}, nil)
		c.mocks.clusterStore.EXPECT().FindNodeByClusterID(ctx, resource.ClusterID).Return([]database.ClusterNode{}, nil)
		c.mocks.workflowStore.EXPECT().CreateWorkFlow(ctx, mock.Anything).Return(nil, errors.New("db error"))

		resp, err := c.CreateJob(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "failed to create ArgoWorkflow record")
	})

	t.Run("deployer create workflow failed", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ResourceId: 100,
			JobName:    "test-job",
		}

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "testuser",
			UUID:   "user-uuid-1",
			NSType: string(database.UserNamespace),
		}

		resource := &database.SpaceResource{
			ID:        100,
			ClusterID: "cluster-1",
			Resources: `{"cpu": {"num": "2"}}`,
		}

		createdWF := &database.ArgoWorkflow{
			ID:       1,
			TaskId:   "df-abc123",
			TaskName: req.JobName,
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)
		c.mocks.userSvcClient.EXPECT().GetOrCreateFirstAvaiTokens(ctx, req.Username, req.Username, string(types.AccessTokenAppGit), "dataflow").Return("test-token", nil)
		c.mocks.spaceResourceStore.EXPECT().FindByID(ctx, req.ResourceId).Return(resource, nil)
		c.mocks.repoComponent.EXPECT().CheckAccountAndResource(ctx, ns.Path, resource.ClusterID, int64(0), resource).Return(&types.CheckExclusiveResp{}, nil)
		c.mocks.clusterStore.EXPECT().FindNodeByClusterID(ctx, resource.ClusterID).Return([]database.ClusterNode{}, nil)
		c.mocks.workflowStore.EXPECT().CreateWorkFlow(ctx, mock.Anything).Return(createdWF, nil)
		c.mocks.deployer.EXPECT().CreateDataflowJob(ctx, mock.Anything).Return(nil, errors.New("deployer error"))
		c.mocks.workflowStore.EXPECT().DeleteWorkFlow(ctx, createdWF.ID).Return(nil)

		resp, err := c.CreateJob(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "failed to create dataflow workflow")
	})

	t.Run("org member has permission", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username:   "orgmember",
			NSUUID:     "org-uuid-1",
			ResourceId: 100,
			JobName:    "test-job",
		}

		user := &types.User{
			UUID:     "user-uuid-member",
			Username: "orgmember",
		}
		ns := &rpc.Namespace{
			Path:   "testorg",
			UUID:   "org-uuid-1",
			NSType: string(database.OrgNamespace),
		}

		resource := &database.SpaceResource{
			ID:        100,
			Name:      "test-resource",
			ClusterID: "cluster-1",
			Resources: `{"cpu": {"num": "2"}}`,
		}

		createdWF := &database.ArgoWorkflow{
			ID:       1,
			TaskId:   "df-abc123",
			TaskName: req.JobName,
		}

		deployResp := &types.DataflowArgoJobResp{
			ArgoTaskID: "df-abc123",
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)
		c.mocks.userSvcClient.EXPECT().GetMemberRoleByUUID(ctx, ns.UUID, req.Username).Return(membership.RoleAdmin, nil)
		c.mocks.userSvcClient.EXPECT().GetOrCreateFirstAvaiTokens(ctx, req.Username, req.Username, string(types.AccessTokenAppGit), "dataflow").Return("test-token", nil)
		c.mocks.spaceResourceStore.EXPECT().FindByID(ctx, req.ResourceId).Return(resource, nil)
		c.mocks.repoComponent.EXPECT().CheckAccountAndResource(ctx, ns.Path, resource.ClusterID, int64(0), resource).Return(&types.CheckExclusiveResp{}, nil)
		c.mocks.clusterStore.EXPECT().FindNodeByClusterID(ctx, resource.ClusterID).Return([]database.ClusterNode{}, nil)
		c.mocks.workflowStore.EXPECT().CreateWorkFlow(ctx, mock.Anything).Return(createdWF, nil)
		c.mocks.deployer.EXPECT().CreateDataflowJob(ctx, mock.Anything).Return(deployResp, nil)

		resp, err := c.CreateJob(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestPlatformDataflowComponent_DeleteJob(t *testing.T) {
	ctx := context.TODO()

	t.Run("success", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowDeleteReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ArgoTaskID: "df-abc123",
		}

		wf := &database.ArgoWorkflow{
			ID:        1,
			TaskId:    req.ArgoTaskID,
			UserUUID:  req.NSUUID,
			ClusterID: "cluster-1",
		}

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "testuser",
			UUID:   "user-uuid-1",
			NSType: string(database.UserNamespace),
		}

		c.mocks.workflowStore.EXPECT().FindByTaskID(ctx, req.ArgoTaskID).Return(wf, nil)
		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)
		c.mocks.deployer.EXPECT().DeleteDataflowJob(ctx, mock.MatchedBy(func(r *types.DataflowArgoReq) bool {
			return r.ArgoTaskID == wf.TaskId && r.ClusterID == wf.ClusterID
		})).Return(nil)
		c.mocks.workflowStore.EXPECT().DeleteWorkFlow(ctx, wf.ID).Return(nil)

		err := c.DeleteJob(ctx, req)
		require.NoError(t, err)
	})

	t.Run("workflow not found", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowDeleteReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ArgoTaskID: "nonexistent",
		}

		c.mocks.workflowStore.EXPECT().FindByTaskID(ctx, req.ArgoTaskID).Return(nil, errors.New("not found"))

		err := c.DeleteJob(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to find dataflow workflow")
	})

	t.Run("permission denied - different user", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowDeleteReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ArgoTaskID: "df-abc123",
		}

		wf := &database.ArgoWorkflow{
			ID:        1,
			TaskId:    req.ArgoTaskID,
			UserUUID:  "different-user-uuid",
			ClusterID: "cluster-1",
		}

		c.mocks.workflowStore.EXPECT().FindByTaskID(ctx, req.ArgoTaskID).Return(wf, nil)

		err := c.DeleteJob(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "do not have permission")
	})

	t.Run("deployer delete failed", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowDeleteReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ArgoTaskID: "df-abc123",
		}

		wf := &database.ArgoWorkflow{
			ID:        1,
			TaskId:    req.ArgoTaskID,
			UserUUID:  req.NSUUID,
			ClusterID: "cluster-1",
		}

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "testuser",
			UUID:   "user-uuid-1",
			NSType: string(database.UserNamespace),
		}

		c.mocks.workflowStore.EXPECT().FindByTaskID(ctx, req.ArgoTaskID).Return(wf, nil)
		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)
		c.mocks.deployer.EXPECT().DeleteDataflowJob(ctx, mock.Anything).Return(errors.New("deployer error"))

		err := c.DeleteJob(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to delete dataflow workflow")
	})

	t.Run("delete workflow record failed", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowDeleteReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ArgoTaskID: "df-abc123",
		}

		wf := &database.ArgoWorkflow{
			ID:        1,
			TaskId:    req.ArgoTaskID,
			UserUUID:  req.NSUUID,
			ClusterID: "cluster-1",
		}

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "testuser",
			UUID:   "user-uuid-1",
			NSType: string(database.UserNamespace),
		}

		c.mocks.workflowStore.EXPECT().FindByTaskID(ctx, req.ArgoTaskID).Return(wf, nil)
		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)
		c.mocks.deployer.EXPECT().DeleteDataflowJob(ctx, mock.Anything).Return(nil)
		c.mocks.workflowStore.EXPECT().DeleteWorkFlow(ctx, wf.ID).Return(errors.New("db error"))

		err := c.DeleteJob(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to delete dataflow workflow record")
	})

	t.Run("user permission check failed", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowDeleteReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ArgoTaskID: "df-abc123",
		}

		wf := &database.ArgoWorkflow{
			ID:        1,
			TaskId:    req.ArgoTaskID,
			UserUUID:  req.NSUUID,
			ClusterID: "cluster-1",
		}

		c.mocks.workflowStore.EXPECT().FindByTaskID(ctx, req.ArgoTaskID).Return(wf, nil)
		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(nil, errors.New("user not found"))

		err := c.DeleteJob(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "current user not found")
	})
}

func TestPlatformDataflowComponent_GetJob(t *testing.T) {
	ctx := context.TODO()

	t.Run("success", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ArgoTaskID: "df-abc123",
		}

		wf := &database.ArgoWorkflow{
			ID:         1,
			TaskId:     req.ArgoTaskID,
			TaskName:   "test-job",
			UserUUID:   req.NSUUID,
			Username:   req.Username,
			Status:     v1alpha1.WorkflowRunning,
			Reason:     "",
			SubmitTime: time.Now(),
			DagTasks:   `{"task1": {"status": "running"}}`,
		}

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "testuser",
			UUID:   "user-uuid-1",
			NSType: string(database.UserNamespace),
		}

		c.mocks.workflowStore.EXPECT().FindByTaskID(ctx, req.ArgoTaskID).Return(wf, nil)
		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)

		resp, err := c.GetJob(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, wf.ID, resp.ID)
		require.Equal(t, wf.TaskId, resp.ArgoTaskID)
		require.Equal(t, wf.TaskName, resp.JobName)
		require.Equal(t, string(wf.Status), resp.Status)
	})

	t.Run("workflow not found", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ArgoTaskID: "nonexistent",
		}

		c.mocks.workflowStore.EXPECT().FindByTaskID(ctx, req.ArgoTaskID).Return(nil, errors.New("not found"))

		resp, err := c.GetJob(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "failed to find dataflow workflow")
	})

	t.Run("permission check failed", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username:   "testuser",
			NSUUID:     "user-uuid-1",
			ArgoTaskID: "df-abc123",
		}

		wf := &database.ArgoWorkflow{
			ID:       1,
			TaskId:   req.ArgoTaskID,
			UserUUID: req.NSUUID,
		}

		c.mocks.workflowStore.EXPECT().FindByTaskID(ctx, req.ArgoTaskID).Return(wf, nil)
		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(nil, errors.New("user not found"))

		resp, err := c.GetJob(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "current user not found")
	})

	t.Run("admin user has permission", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username:   "admin",
			NSUUID:     "user-uuid-1",
			ArgoTaskID: "df-abc123",
		}

		wf := &database.ArgoWorkflow{
			ID:         1,
			TaskId:     req.ArgoTaskID,
			TaskName:   "test-job",
			UserUUID:   "different-user-uuid",
			Username:   "differentuser",
			Status:     v1alpha1.WorkflowSucceeded,
			SubmitTime: time.Now(),
		}

		user := &types.User{
			UUID:     "admin-uuid",
			Username: "admin",
			Roles:    []string{"admin"},
		}
		ns := &rpc.Namespace{
			Path:   "admin",
			UUID:   "admin-uuid",
			NSType: string(database.UserNamespace),
		}

		c.mocks.workflowStore.EXPECT().FindByTaskID(ctx, req.ArgoTaskID).Return(wf, nil)
		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)

		resp, err := c.GetJob(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, wf.ID, resp.ID)
	})

	t.Run("org member has permission", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		req := &types.DataflowArgoJobReq{
			Username:   "orgmember",
			NSUUID:     "org-uuid-1",
			ArgoTaskID: "df-abc123",
		}

		wf := &database.ArgoWorkflow{
			ID:         1,
			TaskId:     req.ArgoTaskID,
			TaskName:   "test-job",
			UserUUID:   req.NSUUID,
			Status:     v1alpha1.WorkflowRunning,
			SubmitTime: time.Now(),
		}

		user := &types.User{
			UUID:     "member-uuid",
			Username: "orgmember",
		}
		ns := &rpc.Namespace{
			Path:   "testorg",
			UUID:   "org-uuid-1",
			NSType: string(database.OrgNamespace),
		}

		c.mocks.workflowStore.EXPECT().FindByTaskID(ctx, req.ArgoTaskID).Return(wf, nil)
		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, req.Username).Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, req.NSUUID).Return(ns, nil)
		c.mocks.userSvcClient.EXPECT().GetMemberRoleByUUID(ctx, ns.UUID, req.Username).Return(membership.RoleWrite, nil)

		resp, err := c.GetJob(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestCheckUserOrOrgPermission(t *testing.T) {
	ctx := context.TODO()

	t.Run("user is admin", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		user := &types.User{
			UUID:     "admin-uuid",
			Username: "admin",
			Roles:    []string{"admin"},
		}
		ns := &rpc.Namespace{
			Path:   "targetuser",
			UUID:   "target-uuid",
			NSType: string(database.UserNamespace),
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, "admin").Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, "target-uuid").Return(ns, nil)

		result, err := checkOwnerOrOrgMemberPermission(ctx, c.userSvcClient, "admin", "target-uuid")
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("user accessing own namespace", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "testuser",
			UUID:   "user-uuid-1",
			NSType: string(database.UserNamespace),
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, "testuser").Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, "user-uuid-1").Return(ns, nil)

		result, err := checkOwnerOrOrgMemberPermission(ctx, c.userSvcClient, "testuser", "user-uuid-1")
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("org member accessing org namespace", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		user := &types.User{
			UUID:     "member-uuid",
			Username: "orgmember",
		}
		ns := &rpc.Namespace{
			Path:   "testorg",
			UUID:   "org-uuid-1",
			NSType: string(database.OrgNamespace),
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, "orgmember").Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, "org-uuid-1").Return(ns, nil)
		c.mocks.userSvcClient.EXPECT().GetMemberRoleByUUID(ctx, "org-uuid-1", "orgmember").Return("member", nil)

		result, err := checkOwnerOrOrgMemberPermission(ctx, c.userSvcClient, "orgmember", "org-uuid-1")
		require.NoError(t, err)
		require.NotNil(t, result)
	})

	t.Run("user accessing other user namespace denied", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "otheruser",
			UUID:   "other-uuid",
			NSType: string(database.UserNamespace),
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, "testuser").Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, "other-uuid").Return(ns, nil)

		result, err := checkOwnerOrOrgMemberPermission(ctx, c.userSvcClient, "testuser", "other-uuid")
		require.Error(t, err)
		require.Contains(t, err.Error(), "do not have permission")
		require.NotNil(t, result)
	})

	t.Run("non-member accessing org namespace denied", func(t *testing.T) {
		c := newTestPlatformDataflowComponent(t)

		user := &types.User{
			UUID:     "user-uuid-1",
			Username: "testuser",
		}
		ns := &rpc.Namespace{
			Path:   "testorg",
			UUID:   "org-uuid-1",
			NSType: string(database.OrgNamespace),
		}

		c.mocks.userSvcClient.EXPECT().GetUserByName(ctx, "testuser").Return(user, nil)
		c.mocks.userSvcClient.EXPECT().GetNameSpaceInfoByUUID(ctx, "org-uuid-1").Return(ns, nil)
		c.mocks.userSvcClient.EXPECT().GetMemberRoleByUUID(ctx, "org-uuid-1", "testuser").Return(membership.RoleUnknown, nil)

		result, err := checkOwnerOrOrgMemberPermission(ctx, c.userSvcClient, "testuser", "org-uuid-1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "do not have permission")
		require.NotNil(t, result)
	})
}
