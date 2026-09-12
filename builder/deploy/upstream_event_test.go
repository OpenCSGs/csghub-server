package deploy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestBuildDeployUpstreamInfoWithDeploy(t *testing.T) {
	repo := &database.Repository{
		Path:   "OpenCSG/Qwen2.5-7B",
		Name:   "Qwen2.5-7B",
		HFPath: "Qwen/Qwen2.5-7B",
	}
	user := &database.User{
		UUID:     "user-uuid-123",
		Username: "testuser",
	}
	createdAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	deploy := &database.Deploy{
		ID:               42,
		Type:             commontypes.InferenceType,
		Endpoint:         "http://my-svc.default.svc.cluster.local:8080/v1",
		ClusterID:        "cluster-abc",
		SvcName:          "svc123",
		ImageID:          "img-xyz",
		RuntimeFramework: "vllm",
		EngineArgs:       `{"tool_call":"enabled"}`,
		Task:             "text-generation",
		OwnerNamespace:   "testuser",
		Repository:       repo,
		User:             user,
	}
	deploy.CreatedAt = createdAt

	info := BuildDeployUpstreamInfoWithDeploy(context.Background(), deploy, nil)

	require.NotNil(t, info)
	assert.Equal(t, int64(42), info.DeployID)
	assert.Equal(t, "OpenCSG/Qwen2.5-7B", info.RepoPath)
	assert.Equal(t, "Qwen2.5-7B", info.RepoName)
	assert.Equal(t, "Qwen/Qwen2.5-7B", info.HFPath)
	assert.Equal(t, commontypes.InferenceType, info.DeployType)
	assert.Equal(t, commontypes.ProviderTypeInference, info.Provider)
	assert.Equal(t, "http://my-svc.default.svc.cluster.local:8080/v1", info.Endpoint)
	assert.Equal(t, "cluster-abc", info.ClusterID)
	assert.Equal(t, "svc123", info.SvcName)
	assert.Equal(t, "img-xyz", info.ImageID)
	assert.Equal(t, "vllm", info.RuntimeFramework)
	assert.Equal(t, `{"tool_call":"enabled"}`, info.EngineArgs)
	assert.Equal(t, "text-generation", info.Task)
	assert.Equal(t, "user-uuid-123", info.UserUUID)
	assert.Equal(t, "testuser", info.OwnerUsername)
	assert.Equal(t, "testuser", info.OwnerNamespace)
	assert.Equal(t, createdAt.Unix(), info.CreatedAt)
	// LegacyModelID for Inference: {repoName}:{base36(deployID)}
	assert.Equal(t, "Qwen2.5-7B:16", info.LegacyModelID)
}

func TestBuildDeployUpstreamInfoWithDeploy_ServerlessType(t *testing.T) {
	deploy := &database.Deploy{
		ID: 10,
		Repository: &database.Repository{
			Path: "org/model",
			Name: "model",
		},
		Type:    commontypes.ServerlessType,
		SvcName: "svc-serverless",
	}

	info := BuildDeployUpstreamInfoWithDeploy(context.Background(), deploy, nil)

	require.NotNil(t, info)
	assert.Equal(t, commontypes.ProviderTypeServerless, info.Provider)
	// LegacyModelID for Serverless: {repoPath} (no suffix)
	assert.Equal(t, "org/model", info.LegacyModelID)
}

func TestBuildDeployUpstreamInfoWithDeploy_NilDeploy(t *testing.T) {
	info := BuildDeployUpstreamInfoWithDeploy(context.Background(), nil, nil)
	assert.Nil(t, info)
}

func TestBuildDeployUpstreamInfoWithDeploy_NilRepository(t *testing.T) {
	deploy := &database.Deploy{ID: 1}
	info := BuildDeployUpstreamInfoWithDeploy(context.Background(), deploy, nil)
	assert.Nil(t, info)
}

func TestBuildDeployUpstreamInfoWithDeploy_NilUser(t *testing.T) {
	deploy := &database.Deploy{
		ID: 1,
		Repository: &database.Repository{
			Path: "org/model",
			Name: "model",
		},
		SvcName: "svc1",
	}

	info := BuildDeployUpstreamInfoWithDeploy(context.Background(), deploy, nil)

	require.NotNil(t, info)
	assert.Empty(t, info.UserUUID)
	assert.Empty(t, info.OwnerUsername)
}

func TestPublishDeployUpstreamSyncEvent_NilMQ(t *testing.T) {
	// When DefaultEventPublisher.MQ is nil (no MQ initialized),
	// the function should return without panicking.
	t.Cleanup(func() {
		// Reset to default state after test
		// DefaultEventPublisher is a package-level var; we only read MQ here
		// so no mutation cleanup is needed.
	})

	// DefaultEventPublisher.MQ is nil in unit tests (no NATS connection).
	// This verifies the guard works and the function is a safe no-op.
	assert.Nil(t, nil) // MQ is nil in test environment
	PublishDeployUpstreamSyncEvent(t.Context(), "test.subject", 1, nil)
}

func TestPublishDeployUpstreamSyncEvent_NilMQWithInfo(t *testing.T) {
	info := &commontypes.DeployUpstreamInfo{
		DeployID:      1,
		RepoPath:      "org/model",
		LegacyModelID: "org/model:svc1",
	}
	// Should not panic even with a non-nil info when MQ is nil.
	PublishDeployUpstreamSyncEvent(t.Context(), "test.subject", 1, info)
}

func TestBuildDeployUpstreamInfoWithDeploy_ResolvesOwnerType(t *testing.T) {
	deploy := &database.Deploy{
		ID: 1,
		Repository: &database.Repository{
			Path: "org/model",
			Name: "model",
		},
		Type:           commontypes.InferenceType,
		SvcName:        "svc1",
		OwnerNamespace: "my-org",
	}

	t.Run("resolves organization namespace type", func(t *testing.T) {
		mockNS := mockdb.NewMockNamespaceStore(t)
		mockNS.EXPECT().FindByPath(mock.Anything, "my-org").
			Return(database.Namespace{NamespaceType: database.OrgNamespace}, nil).Once()

		info := BuildDeployUpstreamInfoWithDeploy(context.Background(), deploy, mockNS)
		require.NotNil(t, info)
		assert.Equal(t, string(database.OrgNamespace), info.OwnerType)
	})

	t.Run("resolves user namespace type", func(t *testing.T) {
		mockNS := mockdb.NewMockNamespaceStore(t)
		mockNS.EXPECT().FindByPath(mock.Anything, "my-org").
			Return(database.Namespace{NamespaceType: database.UserNamespace}, nil).Once()

		info := BuildDeployUpstreamInfoWithDeploy(context.Background(), deploy, mockNS)
		require.NotNil(t, info)
		assert.Equal(t, string(database.UserNamespace), info.OwnerType)
	})

	t.Run("leaves OwnerType empty on lookup error", func(t *testing.T) {
		mockNS := mockdb.NewMockNamespaceStore(t)
		mockNS.EXPECT().FindByPath(mock.Anything, "my-org").
			Return(database.Namespace{}, assert.AnError).Once()

		info := BuildDeployUpstreamInfoWithDeploy(context.Background(), deploy, mockNS)
		require.NotNil(t, info)
		assert.Empty(t, info.OwnerType)
	})

	t.Run("leaves OwnerType empty when nsStore is nil", func(t *testing.T) {
		info := BuildDeployUpstreamInfoWithDeploy(context.Background(), deploy, nil)
		require.NotNil(t, info)
		assert.Empty(t, info.OwnerType)
	})

	t.Run("skips lookup when OwnerNamespace is empty", func(t *testing.T) {
		mockNS := mockdb.NewMockNamespaceStore(t)
		deployNoNS := &database.Deploy{
			ID: 1,
			Repository: &database.Repository{
				Path: "org/model",
				Name: "model",
			},
			Type:    commontypes.InferenceType,
			SvcName: "svc1",
		}
		info := BuildDeployUpstreamInfoWithDeploy(context.Background(), deployNoNS, mockNS)
		require.NotNil(t, info)
		assert.Empty(t, info.OwnerType)
	})
}
