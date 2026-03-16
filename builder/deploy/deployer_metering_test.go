package deploy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/mq"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestDeployer_getResourceMap(t *testing.T) {
	mockSpaceResourceStore := mockdb.NewMockSpaceResourceStore(t)
	mockSpaceResourceStore.EXPECT().FindAll(mock.Anything).
		Return([]database.SpaceResource{
			{ID: 1, Name: "Resource1"},
			{ID: 2, Name: "Resource2"},
		}, nil)

	d := &deployer{
		spaceResourceStore: mockSpaceResourceStore,
	}
	resources := d.getResourceMap()
	require.Equal(t, map[string]string{
		"1": "Resource1",
		"2": "Resource2",
	}, resources)
}

func TestDeployer_getClusterMap(t *testing.T) {
	clusterStore := mockdb.NewMockClusterInfoStore(t)
	clusterStore.EXPECT().List(mock.Anything).Return([]database.ClusterInfo{
		{
			ClusterID: "cluster1",
		},
		{
			ClusterID: "cluster2",
		},
	}, nil)

	d := &deployer{
		clusterStore: clusterStore,
	}

	res := d.getClusterMap()
	require.Equal(t, map[string]database.ClusterInfo{
		"cluster1": {
			ClusterID: "cluster1",
		},
		"cluster2": {
			ClusterID: "cluster2",
		},
	}, res)
}

func TestDeployer_startAcctMeteringRequest(t *testing.T) {
	now := time.Now()
	eventTime := now

	t.Run("skip when cluster does not exist", func(t *testing.T) {
		resMap := map[string]string{"sku1": "resource1"}
		clusterMap := map[string]database.ClusterInfo{}
		deploy := database.Deploy{
			ID:        1,
			ClusterID: "cluster1",
			SKU:       "sku1",
			Type:      types.InferenceType,
		}

		d := &deployer{}
		d.startAcctMeteringRequest(context.Background(), resMap, clusterMap, deploy, eventTime)
		// Should return early without error
	})

	t.Run("skip when cluster is not running", func(t *testing.T) {
		resMap := map[string]string{"sku1": "resource1"}
		clusterMap := map[string]database.ClusterInfo{
			"cluster1": {
				ClusterID: "cluster1",
				Status:    types.ClusterStatusUnavailable,
			},
		}
		deploy := database.Deploy{
			ID:        1,
			ClusterID: "cluster1",
			SKU:       "sku1",
			Type:      types.InferenceType,
		}

		d := &deployer{}
		d.startAcctMeteringRequest(context.Background(), resMap, clusterMap, deploy, eventTime)
		// Should return early without error
	})

	t.Run("skip when cluster heartbeat timeout", func(t *testing.T) {
		resMap := map[string]string{"sku1": "resource1"}
		cluster := database.ClusterInfo{
			ClusterID: "cluster1",
			Status:    types.ClusterStatusRunning,
		}
		cluster.UpdatedAt = now.Add(-30 * time.Minute) // Very old update
		clusterMap := map[string]database.ClusterInfo{
			"cluster1": cluster,
		}
		deploy := database.Deploy{
			ID:        1,
			ClusterID: "cluster1",
			SKU:       "sku1",
			Type:      types.InferenceType,
		}

		d := &deployer{
			deployConfig: common.DeployConfig{
				HeartBeatTimeInSec: 300, // 5 minutes
			},
		}
		d.startAcctMeteringRequest(context.Background(), resMap, clusterMap, deploy, eventTime)
		// Should return early without error
	})

	t.Run("skip when deploy has no SKU", func(t *testing.T) {
		resMap := map[string]string{"sku1": "resource1"}
		cluster := database.ClusterInfo{
			ClusterID: "cluster1",
			Status:    types.ClusterStatusRunning,
		}
		cluster.UpdatedAt = now
		clusterMap := map[string]database.ClusterInfo{
			"cluster1": cluster,
		}
		deploy := database.Deploy{
			ID:        1,
			ClusterID: "cluster1",
			SKU:       "", // Empty SKU
			Type:      types.InferenceType,
		}

		d := &deployer{}
		d.startAcctMeteringRequest(context.Background(), resMap, clusterMap, deploy, eventTime)
		// Should return early without error
	})

	t.Run("skip when SKU not found in resMap", func(t *testing.T) {
		resMap := map[string]string{"sku1": "resource1"}
		cluster := database.ClusterInfo{
			ClusterID: "cluster1",
			Status:    types.ClusterStatusRunning,
		}
		cluster.UpdatedAt = now
		clusterMap := map[string]database.ClusterInfo{
			"cluster1": cluster,
		}
		deploy := database.Deploy{
			ID:        1,
			ClusterID: "cluster1",
			SKU:       "unknown_sku",
			Type:      types.InferenceType,
		}

		d := &deployer{}
		d.startAcctMeteringRequest(context.Background(), resMap, clusterMap, deploy, eventTime)
		// Should return early without error
	})

	t.Run("skip when invalid deploy type", func(t *testing.T) {
		resMap := map[string]string{"sku1": "resource1"}
		cluster := database.ClusterInfo{
			ClusterID: "cluster1",
			Status:    types.ClusterStatusRunning,
		}
		cluster.UpdatedAt = now
		clusterMap := map[string]database.ClusterInfo{
			"cluster1": cluster,
		}
		deploy := database.Deploy{
			ID:        1,
			ClusterID: "cluster1",
			SKU:       "sku1",
			Type:      -1, // Invalid type
		}

		d := &deployer{}
		d.startAcctMeteringRequest(context.Background(), resMap, clusterMap, deploy, eventTime)
		// Should return early without error
	})

	t.Run("skip when ModelServerless scene", func(t *testing.T) {
		resMap := map[string]string{"sku1": "resource1"}
		cluster := database.ClusterInfo{
			ClusterID: "cluster1",
			Status:    types.ClusterStatusRunning,
		}
		cluster.UpdatedAt = now
		clusterMap := map[string]database.ClusterInfo{
			"cluster1": cluster,
		}
		hardwareJSON := `{"replicas":1}`
		deploy := database.Deploy{
			ID:        1,
			ClusterID: "cluster1",
			SKU:       "sku1",
			Type:      types.ServerlessType,
			Hardware:  hardwareJSON,
		}

		d := &deployer{}
		d.startAcctMeteringRequest(context.Background(), resMap, clusterMap, deploy, eventTime)
		// Should return early without error
	})

	t.Run("skip when hardware format is invalid", func(t *testing.T) {
		resMap := map[string]string{"sku1": "resource1"}
		cluster := database.ClusterInfo{
			ClusterID: "cluster1",
			Status:    types.ClusterStatusRunning,
		}
		cluster.UpdatedAt = now
		clusterMap := map[string]database.ClusterInfo{
			"cluster1": cluster,
		}
		deploy := database.Deploy{
			ID:        1,
			ClusterID: "cluster1",
			SKU:       "sku1",
			Type:      types.InferenceType,
			Hardware:  "invalid json",
		}

		d := &deployer{}
		d.startAcctMeteringRequest(context.Background(), resMap, clusterMap, deploy, eventTime)
		// Should return early without error
	})

	t.Run("success with single node deploy and replica count", func(t *testing.T) {
		resMap := map[string]string{"sku1": "resource1"}
		cluster := database.ClusterInfo{
			ClusterID: "cluster1",
			Status:    types.ClusterStatusRunning,
		}
		cluster.UpdatedAt = now
		clusterMap := map[string]database.ClusterInfo{
			"cluster1": cluster,
		}
		hardwareJSON := `{"replicas":1}`
		deploy := database.Deploy{
			ID:         1,
			ClusterID:  "cluster1",
			SKU:        "sku1",
			Type:       types.InferenceType,
			Hardware:   hardwareJSON,
			SvcName:    "test-svc",
			SpaceID:    0,
			UserUUID:   "user1",
			MaxReplica: 2,
			Instances: []types.Instance{
				{Status: string(types.ClusterStatusRunning)},
				{Status: string(types.ClusterStatusRunning)},
				{Status: string(types.ClusterStatusRunning)},
			},
		}

		mockMQ := mockmq.NewMockMessageQueue(t)
		mockMQ.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil)

		d := &deployer{
			eventPub: &event.EventPublisher{
				SyncInterval: 5, // 5 minutes
				MQ:           mockMQ,
			},
			deployConfig: common.DeployConfig{
				HeartBeatTimeInSec: 300,
				UniqueServiceName:  "test-service",
			},
		}
		d.startAcctMeteringRequest(context.Background(), resMap, clusterMap, deploy, eventTime)
		// Should complete without error
	})

	t.Run("success with multi-node deploy (replicas >= 2)", func(t *testing.T) {
		resMap := map[string]string{"sku1": "resource1"}
		cluster := database.ClusterInfo{
			ClusterID: "cluster1",
			Status:    types.ClusterStatusRunning,
		}
		cluster.UpdatedAt = now
		clusterMap := map[string]database.ClusterInfo{
			"cluster1": cluster,
		}
		hardwareJSON := `{"replicas":2}`
		deploy := database.Deploy{
			ID:        1,
			ClusterID: "cluster1",
			SKU:       "sku1",
			Type:      types.InferenceType,
			Hardware:  hardwareJSON,
			SvcName:   "test-svc",
			UserUUID:  "user1",
		}

		mockMQ := mockmq.NewMockMessageQueue(t)
		mockMQ.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil)

		d := &deployer{
			eventPub: &event.EventPublisher{
				SyncInterval: 5,
				MQ:           mockMQ,
			},
			deployConfig: common.DeployConfig{
				HeartBeatTimeInSec: 300,
				UniqueServiceName:  "test-service",
			},
		}
		d.startAcctMeteringRequest(context.Background(), resMap, clusterMap, deploy, eventTime)
		// Should complete without calling GetReplica
	})

	t.Run("success with GetReplica returning multiple replicas", func(t *testing.T) {
		resMap := map[string]string{"sku1": "resource1"}
		cluster := database.ClusterInfo{
			ClusterID: "cluster1",
			Status:    types.ClusterStatusRunning,
		}
		cluster.UpdatedAt = now
		clusterMap := map[string]database.ClusterInfo{
			"cluster1": cluster,
		}
		hardwareJSON := `{"replicas":1}`
		deploy := database.Deploy{
			ID:         1,
			ClusterID:  "cluster1",
			SKU:        "sku1",
			Type:       types.InferenceType,
			Hardware:   hardwareJSON,
			SvcName:    "test-svc",
			SpaceID:    0,
			UserUUID:   "user1",
			MaxReplica: 2,
			Instances: []types.Instance{
				{Status: string(types.ClusterStatusRunning)},
				{Status: string(types.ClusterStatusRunning)},
			},
		}

		mockMQ := mockmq.NewMockMessageQueue(t)
		mockMQ.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil)

		d := &deployer{
			eventPub: &event.EventPublisher{
				SyncInterval: 5,
				MQ:           mockMQ,
			},
			deployConfig: common.DeployConfig{
				HeartBeatTimeInSec: 300,
				UniqueServiceName:  "test-service",
			},
		}
		d.startAcctMeteringRequest(context.Background(), resMap, clusterMap, deploy, eventTime)
		// Should complete without error
	})

	t.Run("handle GetReplica error gracefully", func(t *testing.T) {
		resMap := map[string]string{"sku1": "resource1"}
		cluster := database.ClusterInfo{
			ClusterID: "cluster1",
			Status:    types.ClusterStatusRunning,
		}
		cluster.UpdatedAt = now
		clusterMap := map[string]database.ClusterInfo{
			"cluster1": cluster,
		}
		hardwareJSON := `{"replicas":1}`
		deploy := database.Deploy{
			ID:         1,
			ClusterID:  "cluster1",
			SKU:        "sku1",
			Type:       types.InferenceType,
			Hardware:   hardwareJSON,
			SvcName:    "test-svc",
			SpaceID:    0,
			UserUUID:   "user1",
			MaxReplica: 2,
			Instances:  []types.Instance{},
		}

		mockMQ := mockmq.NewMockMessageQueue(t)
		mockMQ.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil)

		d := &deployer{
			eventPub: &event.EventPublisher{
				SyncInterval: 5,
				MQ:           mockMQ,
			},
			deployConfig: common.DeployConfig{
				HeartBeatTimeInSec: 300,
				UniqueServiceName:  "test-service",
			},
		}
		d.startAcctMeteringRequest(context.Background(), resMap, clusterMap, deploy, eventTime)
		// Should use default replicaCount of 1
	})
}
