package component

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockbldmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/mq"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	deployStatus "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/loki"
	bldmq "opencsg.com/csghub-server/builder/mq"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

var errTestDeployer = errors.New("deployer error")

func TestRepoComponent_DeployInstanceLastLogs_NormalUser(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	logReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "ns",
		Name:        "repo",
		CurrentUser: "test-user",
		DeployID:    1,
		DeployType:  types.InferenceType,
		Limit:       100,
	}

	dbUser := database.User{
		ID:       123,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:          1,
		UserID:      123,
		SvcName:     "svc-1",
		ClusterID:   "cluster-1",
		SecureLevel: types.EndpointPublic,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "test-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)

	lokiResp := &loki.LokiQueryResponse{}
	lokiResp.Status = "success"
	lokiResp.Data.ResultType = "streams"
	lokiResp.Data.Result = []loki.LokiStream{
		{
			Stream: map[string]string{"app": "a"},
			Values: [][]string{
				{"3000000000", "log3"},
				{"1000000000", "log1"},
			},
		},
		{
			Stream: map[string]string{"app": "b"},
			Values: [][]string{
				{"2000000000", "log2"},
				{"4000000000", "log4"},
			},
		},
	}

	repo.mocks.deployer.EXPECT().InstanceLastLogs(ctx, mock.AnythingOfType("types.DeployRequest")).Return(lokiResp, nil)

	result, err := repo.DeployInstanceLastLogs(ctx, logReq)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify merged and sorted by timestamp ascending
	require.Equal(t, 4, len(result.Values))
	require.Equal(t, "1000000000", result.Values[0][0])
	require.Equal(t, "2000000000", result.Values[1][0])
	require.Equal(t, "3000000000", result.Values[2][0])
	require.Equal(t, "4000000000", result.Values[3][0])

	require.Equal(t, "4", result.Stream["count"])
}

func TestRepoComponent_DeployInstanceLastLogs_SingleStream(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	logReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "ns",
		Name:        "repo",
		CurrentUser: "test-user",
		DeployID:    2,
		DeployType:  types.InferenceType,
		Limit:       50,
	}

	dbUser := database.User{
		ID:       123,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:          2,
		UserID:      123,
		SvcName:     "svc-2",
		ClusterID:   "cluster-2",
		SecureLevel: types.EndpointPublic,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "test-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(2)).Return(dbDeploy, nil)

	lokiResp := &loki.LokiQueryResponse{}
	lokiResp.Status = "success"
	lokiResp.Data.ResultType = "streams"
	lokiResp.Data.Result = []loki.LokiStream{
		{
			Stream: map[string]string{"app": "a"},
			Values: [][]string{
				{"3000000000", "log3"},
				{"1000000000", "log1"},
				{"2000000000", "log2"},
			},
		},
	}

	repo.mocks.deployer.EXPECT().InstanceLastLogs(ctx, mock.AnythingOfType("types.DeployRequest")).Return(lokiResp, nil)

	result, err := repo.DeployInstanceLastLogs(ctx, logReq)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 3, len(result.Values))
	require.Equal(t, "1000000000", result.Values[0][0])
	require.Equal(t, "2000000000", result.Values[1][0])
	require.Equal(t, "3000000000", result.Values[2][0])
}

func TestRepoComponent_DeployInstanceLastLogs_EmptyResult(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	logReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "ns",
		Name:        "repo",
		CurrentUser: "test-user",
		DeployID:    3,
		DeployType:  types.InferenceType,
		Limit:       100,
	}

	dbUser := database.User{
		ID:       123,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:          3,
		UserID:      123,
		SvcName:     "svc-3",
		ClusterID:   "cluster-3",
		SecureLevel: types.EndpointPublic,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "test-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(3)).Return(dbDeploy, nil)

	lokiResp := &loki.LokiQueryResponse{}
	lokiResp.Status = "success"
	lokiResp.Data.ResultType = "streams"
	lokiResp.Data.Result = []loki.LokiStream{}

	repo.mocks.deployer.EXPECT().InstanceLastLogs(ctx, mock.AnythingOfType("types.DeployRequest")).Return(lokiResp, nil)

	result, err := repo.DeployInstanceLastLogs(ctx, logReq)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 0, len(result.Values))
	require.Equal(t, "0", result.Stream["count"])
}

func TestRepoComponent_DeployInstanceLastLogs_DeployerError(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	logReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "ns",
		Name:        "repo",
		CurrentUser: "test-user",
		DeployID:    4,
		DeployType:  types.InferenceType,
		Limit:       100,
	}

	dbUser := database.User{
		ID:       123,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:          4,
		UserID:      123,
		SvcName:     "svc-4",
		ClusterID:   "cluster-4",
		SecureLevel: types.EndpointPublic,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "test-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(4)).Return(dbDeploy, nil)

	repo.mocks.deployer.EXPECT().InstanceLastLogs(ctx, mock.AnythingOfType("types.DeployRequest")).Return(nil, errTestDeployer)

	result, err := repo.DeployInstanceLastLogs(ctx, logReq)
	require.Error(t, err)
	require.Nil(t, result)
}

func TestCheckDeployPermissionForUser_ZeroSecureLevel_OwnerAllowed(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	dbUser := database.User{
		ID:       123,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:          1,
		UserID:      123,
		SvcName:     "svc-1",
		ClusterID:   "cluster-1",
		SecureLevel: 0,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "owner-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)

	user, deploy, err := repo.CheckDeployPermissionForUser(ctx, types.DeployActReq{
		CurrentUser: "owner-user",
		DeployID:    1,
	})
	require.NoError(t, err)
	require.NotNil(t, user)
	require.NotNil(t, deploy)
	require.Equal(t, int64(123), user.ID)
}

func TestCheckDeployPermissionForUser_ZeroSecureLevel_NonOwnerForbidden(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	repo.orgStore = repo.mocks.stores.Org

	dbUser := database.User{
		ID:       456,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:          1,
		UserID:      123,
		SvcName:     "svc-1",
		ClusterID:   "cluster-1",
		SecureLevel: 0,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "other-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)
	repo.mocks.stores.OrgMock().EXPECT().GetSharedOrgIDs(ctx, []int64{456, 123}).Return([]int64{}, nil)

	user, deploy, err := repo.CheckDeployPermissionForUser(ctx, types.DeployActReq{
		CurrentUser: "other-user",
		DeployID:    1,
	})
	require.Error(t, err)
	require.Nil(t, user)
	require.Nil(t, deploy)
}

func TestCheckDeployPermissionForUser_PrivateEndpoint_OwnerAllowed(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	dbUser := database.User{
		ID:       123,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:          1,
		UserID:      123,
		SvcName:     "svc-1",
		ClusterID:   "cluster-1",
		SecureLevel: types.EndpointPrivate,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "owner-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)

	user, deploy, err := repo.CheckDeployPermissionForUser(ctx, types.DeployActReq{
		CurrentUser: "owner-user",
		DeployID:    1,
	})
	require.NoError(t, err)
	require.NotNil(t, user)
	require.NotNil(t, deploy)
}

func TestCheckDeployPermissionForUser_PrivateEndpoint_AdminForbidden(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	dbUser := database.User{
		ID:       456,
		RoleMask: "admin",
	}
	dbDeploy := &database.Deploy{
		ID:          1,
		UserID:      123,
		SvcName:     "svc-1",
		ClusterID:   "cluster-1",
		SecureLevel: types.EndpointPrivate,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)

	user, deploy, err := repo.CheckDeployPermissionForUser(ctx, types.DeployActReq{
		CurrentUser: "admin-user",
		DeployID:    1,
	})
	require.Error(t, err)
	require.Nil(t, user)
	require.Nil(t, deploy)
}

func TestCheckDeployPermissionForUser_PublicEndpoint_OwnerAllowed(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	dbUser := database.User{
		ID:       123,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:          1,
		UserID:      123,
		SvcName:     "svc-1",
		ClusterID:   "cluster-1",
		SecureLevel: types.EndpointPublic,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "owner-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)

	user, deploy, err := repo.CheckDeployPermissionForUser(ctx, types.DeployActReq{
		CurrentUser: "owner-user",
		DeployID:    1,
	})
	require.NoError(t, err)
	require.NotNil(t, user)
	require.NotNil(t, deploy)
	require.Equal(t, int64(123), user.ID)
}

func TestCheckDeployPermissionForUser_PublicEndpoint_AdminAllowed(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	dbUser := database.User{
		ID:       456,
		RoleMask: "admin",
	}
	dbDeploy := &database.Deploy{
		ID:          1,
		UserID:      123,
		SvcName:     "svc-1",
		ClusterID:   "cluster-1",
		SecureLevel: types.EndpointPublic,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)

	user, deploy, err := repo.CheckDeployPermissionForUser(ctx, types.DeployActReq{
		CurrentUser: "admin-user",
		DeployID:    1,
	})
	require.NoError(t, err)
	require.NotNil(t, user)
	require.NotNil(t, deploy)
}

func TestCheckDeployPermissionForUser_PublicEndpoint_SameOrgAllowed(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	repo.orgStore = repo.mocks.stores.Org

	dbUser := database.User{
		ID:       456,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:          1,
		UserID:      123,
		SvcName:     "svc-1",
		ClusterID:   "cluster-1",
		SecureLevel: types.EndpointPublic,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "org-member").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)
	repo.mocks.stores.OrgMock().EXPECT().GetSharedOrgIDs(ctx, []int64{456, 123}).Return([]int64{10}, nil)

	user, deploy, err := repo.CheckDeployPermissionForUser(ctx, types.DeployActReq{
		CurrentUser: "org-member",
		DeployID:    1,
	})
	require.NoError(t, err)
	require.NotNil(t, user)
	require.NotNil(t, deploy)
}

func TestCheckDeployPermissionForUser_PublicEndpoint_DifferentOrgForbidden(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)
	repo.orgStore = repo.mocks.stores.Org

	dbUser := database.User{
		ID:       456,
		RoleMask: "",
	}
	dbDeploy := &database.Deploy{
		ID:          1,
		UserID:      123,
		SvcName:     "svc-1",
		ClusterID:   "cluster-1",
		SecureLevel: types.EndpointPublic,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "other-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)
	repo.mocks.stores.OrgMock().EXPECT().GetSharedOrgIDs(ctx, []int64{456, 123}).Return([]int64{}, nil)

	user, deploy, err := repo.CheckDeployPermissionForUser(ctx, types.DeployActReq{
		CurrentUser: "other-user",
		DeployID:    1,
	})
	require.Error(t, err)
	require.Nil(t, user)
	require.Nil(t, deploy)
}

func TestRepoComponent_DeployInstanceLastLogs_ServerlessType(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	logReq := types.DeployActReq{
		RepoType:    types.ModelRepo,
		Namespace:   "ns",
		Name:        "repo",
		CurrentUser: "admin-user",
		DeployID:    6,
		DeployType:  types.ServerlessType,
		Limit:       100,
	}

	dbUser := database.User{
		ID:       1,
		RoleMask: "admin",
	}
	dbDeploy := &database.Deploy{
		ID:          6,
		UserID:      1,
		SvcName:     "svc-6",
		ClusterID:   "cluster-6",
		SecureLevel: types.EndpointPublic,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "admin-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(6)).Return(dbDeploy, nil)

	lokiResp := &loki.LokiQueryResponse{}
	lokiResp.Status = "success"
	lokiResp.Data.ResultType = "streams"
	lokiResp.Data.Result = []loki.LokiStream{
		{
			Values: [][]string{
				{"5000000000", "log5"},
			},
		},
	}

	repo.mocks.deployer.EXPECT().InstanceLastLogs(ctx, mock.AnythingOfType("types.DeployRequest")).Return(lokiResp, nil)

	result, err := repo.DeployInstanceLastLogs(ctx, logReq)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, len(result.Values))
	require.Equal(t, "5000000000", result.Values[0][0])
	require.Equal(t, "1", result.Stream["count"])
}

// =====================================================================
// DeployUpdate → upstream sync event
// =====================================================================

// After a successful DeployUpdate of a running deploy, a running-sync event
// must be published so changed deploy fields (endpoint, cluster, secure
// level...) reach the AIGateway upstream store without waiting for the
// periodic reconcile pass.
func TestRepoComponent_DeployUpdate_PublishesUpstreamSyncEventForRunningDeploy(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	dbUser := database.User{ID: 123}
	dbDeploy := &database.Deploy{
		ID:          1,
		UserID:      123,
		SvcName:     "svc-1",
		ClusterID:   "cluster-1",
		Status:      deployStatus.Running,
		Type:        types.InferenceType,
		SecureLevel: types.EndpointPrivate,
	}
	// The goroutine re-loads the persisted deploy with relations before
	// building the upstream info.
	fullDeploy := &database.Deploy{
		ID:          1,
		UserID:      123,
		SvcName:     "svc-1",
		ClusterID:   "cluster-1",
		Status:      deployStatus.Running,
		SecureLevel: types.EndpointPublic,
		Repository: &database.Repository{
			Path: "ns/model-1",
			Name: "model-1",
		},
		User: &dbUser,
	}

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "owner-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)
	repo.mocks.deployer.EXPECT().UpdateDeploy(ctx, mock.Anything, dbDeploy).Return(nil)

	var relationsLoaded atomic.Bool
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByIDWithRelations(mock.Anything, int64(1)).
		RunAndReturn(func(ctx context.Context, deployID int64) (*database.Deploy, error) {
			relationsLoaded.Store(true)
			return fullDeploy, nil
		}).Once()

	newLevel := types.EndpointPublic
	err := repo.DeployUpdate(ctx, types.DeployActReq{
		CurrentUser: "owner-user",
		DeployID:    1,
		DeployType:  types.InferenceType,
	}, &types.DeployUpdateReq{SecureLevel: &newLevel})
	require.NoError(t, err)

	// The sync runs in a goroutine — wait for the relations load, which is
	// the last observable store interaction before the (no-op in tests)
	// publish.
	assert.Eventually(t, relationsLoaded.Load, 2*time.Second, 10*time.Millisecond,
		"running-sync goroutine should reload the deploy with relations")
}

// Updating a stopped deploy must not publish a running-sync event: its
// upstream was already disabled by the stop event, and a running event would
// wrongly re-enable it.
func TestRepoComponent_DeployUpdate_SkipsUpstreamSyncForStoppedDeploy(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	dbUser := database.User{ID: 123}
	dbDeploy := &database.Deploy{
		ID:          1,
		UserID:      123,
		SvcName:     "svc-1",
		ClusterID:   "cluster-1",
		Status:      deployStatus.Stopped,
		SecureLevel: types.EndpointPrivate,
	}

	var relationsLoaded atomic.Bool
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByIDWithRelations(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, deployID int64) (*database.Deploy, error) {
			relationsLoaded.Store(true)
			return nil, nil
		}).Maybe()

	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "owner-user").Return(dbUser, nil)
	repo.mocks.stores.DeployTaskMock().EXPECT().GetDeployByID(ctx, int64(1)).Return(dbDeploy, nil)
	repo.mocks.deployer.EXPECT().UpdateDeploy(ctx, mock.Anything, dbDeploy).Return(nil)

	newLevel := types.EndpointPublic
	err := repo.DeployUpdate(ctx, types.DeployActReq{
		CurrentUser: "owner-user",
		DeployID:    1,
		DeployType:  types.InferenceType,
	}, &types.DeployUpdateReq{SecureLevel: &newLevel})
	require.NoError(t, err)

	// Give a wrongly spawned goroutine a chance to run before asserting.
	time.Sleep(150 * time.Millisecond)
	assert.False(t, relationsLoaded.Load(), "stopped deploy must not trigger an upstream sync")
}

// Direct tests for the sync helper: gated on syncable deploy type and
// running/sleeping status (checked both at call time and against the
// reloaded DB row), resilient to store errors and deploys without a
// repository. The MQ publisher is swapped in so tests observe the actual
// publish call instead of a silent no-op.
func TestPublishUpstreamSyncAfterUpdate(t *testing.T) {
	ctx := context.TODO()

	// swapTestMQ replaces the package-level publisher's MQ with a strict mock
	// for the duration of the test; any unexpected Publish fails the test.
	swapTestMQ := func(t *testing.T) *mockbldmq.MockMessageQueue {
		t.Helper()
		orig := event.DefaultEventPublisher.MQ
		t.Cleanup(func() { event.DefaultEventPublisher.MQ = orig })
		mq := mockbldmq.NewMockMessageQueue(t)
		event.DefaultEventPublisher.MQ = mq
		return mq
	}

	// expectPublish registers a Publish expectation that records the call in
	// an atomic flag; the publish runs in a goroutine inside
	// EventPublisher.PublishDeployUpstreamSyncEvent, so tests must poll.
	expectPublish := func(t *testing.T, mq *mockbldmq.MockMessageQueue) *atomic.Bool {
		t.Helper()
		var published atomic.Bool
		mq.EXPECT().Publish(bldmq.DeployUpstreamSyncRunningSubject, mock.Anything).
			RunAndReturn(func(topic string, data []byte) error {
				published.Store(true)
				return nil
			}).Once()
		return &published
	}

	// noPublish gives a wrongly spawned async publisher time to hit the
	// strict mock (which fails the test on the unexpected call).
	noPublish := func() {
		time.Sleep(150 * time.Millisecond)
	}

	newRepoComponent := func(t *testing.T) (*repoComponentImpl, *mockdb.MockDeployTaskStore) {
		t.Helper()
		repo := initializeTestRepoComponent(ctx, t)
		return repo.repoComponentImpl, repo.mocks.stores.DeployTaskMock()
	}

	runningFullDeploy := func(deployType int) *database.Deploy {
		return &database.Deploy{
			ID:         1,
			Status:     deployStatus.Running,
			Type:       deployType,
			User:       &database.User{UUID: "uuid-1", Username: "user1"},
			Repository: &database.Repository{Path: "ns/model", Name: "model"},
		}
	}

	t.Run("running inference deploy reloads relations and publishes", func(t *testing.T) {
		comp, mockDeploy := newRepoComponent(t)
		mockMQ := swapTestMQ(t)
		mockDeploy.EXPECT().GetDeployByIDWithRelations(mock.Anything, int64(1)).Return(runningFullDeploy(types.InferenceType), nil).Once()
		published := expectPublish(t, mockMQ)

		comp.publishUpstreamSyncAfterUpdate(ctx, &database.Deploy{ID: 1, Status: deployStatus.Running, Type: types.InferenceType})
		require.True(t, mockDeploy.AssertExpectations(t))
		assert.Eventually(t, published.Load, 2*time.Second, 10*time.Millisecond, "running-sync event should be published")
	})

	t.Run("sleeping serverless deploy reloads relations and publishes", func(t *testing.T) {
		comp, mockDeploy := newRepoComponent(t)
		mockMQ := swapTestMQ(t)
		fullDeploy := runningFullDeploy(types.ServerlessType)
		fullDeploy.Status = deployStatus.Sleeping
		mockDeploy.EXPECT().GetDeployByIDWithRelations(mock.Anything, int64(1)).Return(fullDeploy, nil).Once()
		published := expectPublish(t, mockMQ)

		comp.publishUpstreamSyncAfterUpdate(ctx, &database.Deploy{ID: 1, Status: deployStatus.Sleeping, Type: types.ServerlessType})
		require.True(t, mockDeploy.AssertExpectations(t))
		assert.Eventually(t, published.Load, 2*time.Second, 10*time.Millisecond, "running-sync event should be published")
	})

	t.Run("non-syncable deploy type is skipped without store access", func(t *testing.T) {
		comp, mockDeploy := newRepoComponent(t)
		swapTestMQ(t)

		// Zero expectations: spaces, finetunes, evaluations, and notebooks are
		// not synced to the AIGateway upstream store, even when running.
		comp.publishUpstreamSyncAfterUpdate(ctx, &database.Deploy{ID: 1, Status: deployStatus.Running, Type: types.SpaceType})
		noPublish()
		require.True(t, mockDeploy.AssertExpectations(t))
	})

	t.Run("stopped deploy is skipped without store access", func(t *testing.T) {
		comp, mockDeploy := newRepoComponent(t)
		swapTestMQ(t)

		// With zero expectations set, any store call would fail the test via
		// the strict mock; AssertExpectations confirms none were made.
		comp.publishUpstreamSyncAfterUpdate(ctx, &database.Deploy{ID: 1, Status: deployStatus.Stopped, Type: types.InferenceType})
		noPublish()
		require.True(t, mockDeploy.AssertExpectations(t))
	})

	// The deploy may be stopped or deleted between the update request and the
	// async sync — the reloaded DB row's status must be re-checked before
	// publishing, otherwise a stopped deploy's upstream would be re-enabled.
	t.Run("deploy stopped in DB before sync is skipped without publish", func(t *testing.T) {
		comp, mockDeploy := newRepoComponent(t)
		swapTestMQ(t)
		stoppedInDB := runningFullDeploy(types.InferenceType)
		stoppedInDB.Status = deployStatus.Stopped
		mockDeploy.EXPECT().GetDeployByIDWithRelations(mock.Anything, int64(1)).Return(stoppedInDB, nil).Once()
		// No Publish expectation: publishing would fail the strict mock.

		comp.publishUpstreamSyncAfterUpdate(ctx, &database.Deploy{ID: 1, Status: deployStatus.Running, Type: types.InferenceType})
		noPublish()
		require.True(t, mockDeploy.AssertExpectations(t))
	})

	t.Run("store error is logged and swallowed", func(t *testing.T) {
		comp, mockDeploy := newRepoComponent(t)
		swapTestMQ(t)
		mockDeploy.EXPECT().GetDeployByIDWithRelations(mock.Anything, int64(1)).Return(nil, errors.New("db down")).Once()

		comp.publishUpstreamSyncAfterUpdate(ctx, &database.Deploy{ID: 1, Status: deployStatus.Running, Type: types.InferenceType})
		noPublish()
		require.True(t, mockDeploy.AssertExpectations(t))
	})

	t.Run("deploy without repository is skipped", func(t *testing.T) {
		comp, mockDeploy := newRepoComponent(t)
		swapTestMQ(t)
		fullDeploy := &database.Deploy{ID: 1, Status: deployStatus.Running, Type: types.InferenceType}
		mockDeploy.EXPECT().GetDeployByIDWithRelations(mock.Anything, int64(1)).Return(fullDeploy, nil).Once()

		comp.publishUpstreamSyncAfterUpdate(ctx, &database.Deploy{ID: 1, Status: deployStatus.Running, Type: types.InferenceType})
		noPublish()
		require.True(t, mockDeploy.AssertExpectations(t))
	})
}
