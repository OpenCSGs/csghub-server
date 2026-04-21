package deploy

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mockReporter "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/reporter"
	mockSender "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/reporter/sender"

	"github.com/bwmarrin/snowflake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockacct "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/accounting"
	mockbuilder "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/imagebuilder"
	mockrunner "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/imagerunner"
	mockScheduler "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/scheduler"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"

	corev1 "k8s.io/api/core/v1"
)

type testDepolyerWithMocks struct {
	*deployer
	mocks struct {
		stores    *tests.MockStores
		scheduler *mockScheduler.MockScheduler
		builder   *mockbuilder.MockBuilder
		runner    *mockrunner.MockRunner
		acctClent *mockacct.MockAccountingClient
	}
}

func TestDeployer_GenerateUniqueSvcName(t *testing.T) {
	dr := types.DeployRequest{
		Path: "namespace/name",
	}

	node, _ := snowflake.NewNode(1)
	d := &deployer{
		snowflakeNode: node,
		logReporter:   mockReporter.NewMockLogCollector(t),
	}

	dr.Type = types.SpaceType
	name := d.GenerateUniqueSvcName(dr)
	require.True(t, strings.HasPrefix(name, "u"))

	dr.Type = types.ServerlessType
	name = d.GenerateUniqueSvcName(dr)
	require.True(t, strings.HasPrefix(name, "s"))

	dr.Type = types.InferenceType
	name = d.GenerateUniqueSvcName(dr)
	require.False(t, strings.Contains(name, "-"))

}

func TestDeployer_serverlessDeploy(t *testing.T) {
	t.Run("deploy space", func(t *testing.T) {
		var oldDeploy database.Deploy
		oldDeploy.ID = 1

		dr := types.DeployRequest{
			SpaceID:          1,
			Type:             types.SpaceType,
			UserUUID:         "1",
			SKU:              "1",
			ImageID:          "image:1",
			Annotation:       "test annotation",
			Env:              "test env",
			RuntimeFramework: "test runtime framework",
			Secret:           "test secret",
			SecureLevel:      1,
			ContainerPort:    8000,
			Template:         "test template",
			MinReplica:       1,
			MaxReplica:       2,
			DeployExtend: types.DeployExtend{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "foo",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"bar"},
									},
								},
							},
						},
					},
				},
				Tolerations: []types.Toleration{
					{
						Key:      "foo",
						Operator: "Equal",
						Value:    "bar",
						Effect:   "NoSchedule",
					},
				},
			},
		}

		newDeploy := oldDeploy
		newDeploy.UserUUID = dr.UserUUID
		newDeploy.SKU = dr.SKU
		newDeploy.ImageID = dr.ImageID
		newDeploy.Annotation = dr.Annotation
		newDeploy.Env = dr.Env
		newDeploy.RuntimeFramework = dr.RuntimeFramework
		newDeploy.Secret = dr.Secret
		newDeploy.SecureLevel = dr.SecureLevel
		newDeploy.ContainerPort = dr.ContainerPort
		newDeploy.Template = dr.Template
		newDeploy.MinReplica = dr.MinReplica
		newDeploy.MaxReplica = dr.MaxReplica
		newDeploy.NodeAffinity = dr.NodeAffinity
		newDeploy.Tolerations = dr.Tolerations

		mockTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockTaskStore.EXPECT().GetLatestDeployBySpaceID(mock.Anything, dr.SpaceID).Return(&oldDeploy, nil)
		mockTaskStore.EXPECT().UpdateDeploy(mock.Anything, &newDeploy).Return(nil)

		d := &deployer{
			deployTaskStore: mockTaskStore,
		}
		dbdeploy, err := d.serverlessDeploy(context.TODO(), dr)
		require.Nil(t, err)
		require.Equal(t, *dbdeploy, newDeploy)
	})

	t.Run("deploy model", func(t *testing.T) {
		var oldDeploy database.Deploy
		oldDeploy.ID = 1

		dr := types.DeployRequest{
			RepoID:           1,
			Type:             types.InferenceType,
			UserUUID:         "1",
			SKU:              "1",
			ImageID:          "image:1",
			Annotation:       "test annotation",
			Env:              "test env",
			RuntimeFramework: "test runtime framework",
			Secret:           "test secret",
			SecureLevel:      1,
			ContainerPort:    8000,
			Template:         "test template",
			MinReplica:       1,
			MaxReplica:       2,
			DeployExtend: types.DeployExtend{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "foo",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"bar"},
									},
								},
							},
						},
					},
				},
				Tolerations: []types.Toleration{
					{
						Key:      "foo",
						Operator: "Equal",
						Value:    "bar",
						Effect:   "NoSchedule",
					},
				},
			},
		}

		newDeploy := oldDeploy
		newDeploy.UserUUID = dr.UserUUID
		newDeploy.SKU = dr.SKU
		newDeploy.ImageID = dr.ImageID
		newDeploy.Annotation = dr.Annotation
		newDeploy.Env = dr.Env
		newDeploy.RuntimeFramework = dr.RuntimeFramework
		newDeploy.Secret = dr.Secret
		newDeploy.SecureLevel = dr.SecureLevel
		newDeploy.ContainerPort = dr.ContainerPort
		newDeploy.Template = dr.Template
		newDeploy.MinReplica = dr.MinReplica
		newDeploy.MaxReplica = dr.MaxReplica
		newDeploy.NodeAffinity = dr.NodeAffinity
		newDeploy.Tolerations = dr.Tolerations

		mockTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockTaskStore.EXPECT().GetServerlessDeployByRepID(mock.Anything, dr.RepoID).Return(&oldDeploy, nil)
		mockTaskStore.EXPECT().UpdateDeploy(mock.Anything, &newDeploy).Return(nil)

		d := &deployer{
			deployTaskStore: mockTaskStore,
		}
		dbdeploy, err := d.serverlessDeploy(context.TODO(), dr)
		require.Nil(t, err)
		require.Equal(t, *dbdeploy, newDeploy)
	})
}

func TestDeployer_dedicatedDeploy(t *testing.T) {
	dr := types.DeployRequest{
		Path: "namespace/name",
		Type: types.InferenceType,
	}

	mockTaskStore := mockdb.NewMockDeployTaskStore(t)
	mockTaskStore.EXPECT().CreateDeploy(mock.Anything, mock.Anything).Return(nil)

	node, _ := snowflake.NewNode(1)
	d := &deployer{
		snowflakeNode:   node,
		deployTaskStore: mockTaskStore,
	}

	_, err := d.dedicatedDeploy(context.TODO(), dr)
	require.Nil(t, err)

}

func TestDeployer_Deploy(t *testing.T) {
	DeployWorkflow = func(buildTask, runTask *database.DeployTask) {}
	t.Run("use on-demand resource and skip build task", func(t *testing.T) {
		dr := types.DeployRequest{
			UserUUID: "1",
			Path:     "namespace/name",
			Type:     types.InferenceType,
			ImageID:  "image:1",
		}

		buildTask := database.DeployTask{
			TaskType: 0,
			Status:   common.TaskStatusBuildSkip,
			Message:  "Skip",
		}
		runTask := database.DeployTask{
			TaskType: 1,
		}

		mockTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockTaskStore.EXPECT().CreateDeploy(mock.Anything, mock.Anything).Return(nil)
		mockTaskStore.EXPECT().CreateDeployTask(mock.Anything, &buildTask).Return(nil)
		// RunAndReturn(func(ctx context.Context, dt *database.DeployTask) error {
		// 	return nil
		// })
		mockTaskStore.EXPECT().CreateDeployTask(mock.Anything, &runTask).Return(nil)

		// mockSch := mockScheduler.NewMockScheduler(t)
		// mockSch.EXPECT().Queue(mock.Anything).Return(nil)

		node, _ := snowflake.NewNode(1)

		reporter := mockReporter.NewMockLogCollector(t)
		d := &deployer{
			snowflakeNode:   node,
			deployTaskStore: mockTaskStore,
			// scheduler:       mockSch,
			logReporter: reporter,
		}

		reporter.EXPECT().Report(mock.Anything)

		_, err := d.Deploy(context.TODO(), dr)
		// wait for scheduler.Queue to be called
		time.Sleep(time.Second)
		require.Nil(t, err)
	})
}

func TestDeployer_Status(t *testing.T) {
	t.Run("no deploy", func(t *testing.T) {
		dr := types.DeployRequest{
			UserUUID: "1",
			Path:     "namespace/name",
			Type:     types.InferenceType,
		}

		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockDeployTaskStore.EXPECT().GetDeployByID(mock.Anything, dr.DeployID).
			Return(nil, nil)
		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
		}

		svcName, deployStatus, instances, err := d.Status(context.TODO(), dr, false)
		require.NotNil(t, err)
		require.Equal(t, "", svcName)
		require.Equal(t, common.Stopped, deployStatus)
		require.Nil(t, instances)

	})
	t.Run("cache miss and running", func(t *testing.T) {
		dr := types.DeployRequest{
			DeployID:  1,
			UserUUID:  "1",
			Path:      "namespace/name",
			Type:      types.InferenceType,
			ClusterID: "test",
		}
		deploy := &database.Deploy{
			Status:    common.Building,
			SvcName:   "svc",
			ClusterID: "test",
			Instances: nil,
		}
		mockClusterInfoStore := mockdb.NewMockClusterInfoStore(t)
		mockClusterInfoStore.EXPECT().GetClusterResources(mock.Anything, "test").
			Return(&types.ClusterRes{
				Enable:         true,
				LastUpdateTime: time.Now().Unix(),
				Status:         types.ClusterStatusRunning,
			}, nil)
		cfg := config.Config{}
		cfg.Runner.HearBeatIntervalInSec = 120
		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockDeployTaskStore.EXPECT().GetDeployByID(mock.Anything, dr.DeployID).
			Return(deploy, nil)

		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
			clusterStore:    mockClusterInfoStore,
			config:          &cfg,
		}

		svcName, deployStatus, instances, err := d.Status(context.TODO(), dr, false)
		require.Nil(t, err)
		require.Equal(t, "svc", svcName)
		require.Equal(t, common.Building, deployStatus)
		require.Nil(t, instances)

	})

	t.Run("cache miss and not running", func(t *testing.T) {
		dr := types.DeployRequest{
			DeployID:  1,
			UserUUID:  "1",
			Path:      "namespace/name",
			Type:      types.InferenceType,
			ClusterID: "test",
		}
		deploy := &database.Deploy{
			Status:    common.BuildSuccess,
			SvcName:   "svc",
			ClusterID: "test",
			Instances: nil,
		}
		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockDeployTaskStore.EXPECT().GetDeployByID(mock.Anything, dr.DeployID).
			Return(deploy, nil)
		mockClusterInfoStore := mockdb.NewMockClusterInfoStore(t)
		mockClusterInfoStore.EXPECT().GetClusterResources(mock.Anything, "test").
			Return(&types.ClusterRes{
				Enable:         true,
				LastUpdateTime: time.Now().Unix(),
				Status:         types.ClusterStatusRunning,
			}, nil)
		cfg := config.Config{}
		cfg.Runner.HearBeatIntervalInSec = 120
		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
			clusterStore:    mockClusterInfoStore,
			config:          &cfg,
		}

		svcName, deployStatus, instances, err := d.Status(context.TODO(), dr, false)
		require.Nil(t, err)
		require.Equal(t, "svc", svcName)
		require.Equal(t, common.BuildSuccess, deployStatus)
		require.Nil(t, instances)

	})

	t.Run("cache hit and running", func(t *testing.T) {
		dr := types.DeployRequest{
			DeployID:  1,
			UserUUID:  "1",
			Path:      "namespace/name",
			Type:      types.InferenceType,
			ModelID:   1,
			ClusterID: "test",
		}
		// build success status in db
		deploy := &database.Deploy{
			Status:    common.BuildSuccess,
			SvcName:   "svc",
			ClusterID: "test",
			Instances: []types.Instance{{
				Name: "instance1",
			}},
		}

		mockClusterInfoStore := mockdb.NewMockClusterInfoStore(t)
		mockClusterInfoStore.EXPECT().GetClusterResources(mock.Anything, "test").
			Return(&types.ClusterRes{
				Enable:         true,
				LastUpdateTime: time.Now().Unix(),
				Status:         types.ClusterStatusRunning,
			}, nil)
		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockDeployTaskStore.EXPECT().GetDeployByID(mock.Anything, dr.DeployID).
			Return(deploy, nil)

		cfg := config.Config{}
		cfg.Runner.HearBeatIntervalInSec = 120
		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
			clusterStore:    mockClusterInfoStore,
			config:          &cfg,
		}

		svcName, deployStatus, instances, err := d.Status(context.TODO(), dr, false)
		require.Nil(t, err)
		require.Equal(t, "svc", svcName)
		require.Equal(t, common.BuildSuccess, deployStatus)
		require.Len(t, instances, 1)

	})
}

func TestDeployer_Logs(t *testing.T) {
	t.Run("no deploy", func(t *testing.T) {
		dr := types.DeployRequest{
			UserUUID: "1",
			Path:     "namespace/name",
			Type:     types.InferenceType,
			SpaceID:  1,
		}

		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockDeployTaskStore.EXPECT().GetLatestDeployBySpaceID(mock.Anything, dr.SpaceID).
			Return(nil, sql.ErrNoRows)

		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
		}

		lreader, err := d.Logs(context.TODO(), dr)
		require.NotNil(t, err)
		require.Nil(t, lreader)

	})
	t.Run("get log reader", func(t *testing.T) {
		dr := types.DeployRequest{
			SpaceID:  1,
			DeployID: 1,
			UserUUID: "1",
			Path:     "namespace/name",
			Type:     types.InferenceType,
		}
		deploy := &database.Deploy{
			Status:    common.Running,
			SvcName:   "svc",
			ClusterID: "111",
		}

		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockDeployTaskStore.EXPECT().GetLatestDeployBySpaceID(mock.Anything, dr.SpaceID).
			Return(deploy, nil)

		mockBuilder := mockbuilder.NewMockBuilder(t)
		logReporter := mockReporter.NewMockLogCollector(t)
		sender := mockSender.NewMockLogSender(t)

		mockDeployTaskStore.EXPECT().GetLatestDeployBySpaceID(mock.Anything, dr.DeployID).
			Return(deploy, nil)

		buildTask := &database.DeployTask{
			ID: 1,
		}
		runTask := &database.DeployTask{
			ID: 2,
		}
		tasks := []*database.DeployTask{
			buildTask,
			runTask,
		}
		mockDeployTaskStore.EXPECT().GetDeployTasksOfDeploy(mock.Anything, deploy.ID).
			Return(tasks, nil)

		ch := make(chan string)
		sender.EXPECT().StreamAllLogs(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(ch, nil)
		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
			imageBuilder:    mockBuilder,
			logReporter:     logReporter,
			lokiClient:      sender,
		}

		lreader, err := d.Logs(context.TODO(), dr)
		require.Nil(t, err)
		require.NotNil(t, lreader)
	})
}

func TestDeployer_Purge(t *testing.T) {
	dr := types.DeployRequest{
		SpaceID:  0,
		DeployID: 1,
		UserUUID: "1",
		Path:     "namespace/name",
		Type:     types.InferenceType,
	}

	mockRunner := mockrunner.NewMockRunner(t)
	mockRunner.EXPECT().Purge(mock.Anything, mock.Anything).Return(&types.PurgeResponse{}, nil)

	d := &deployer{
		imageRunner: mockRunner,
	}
	err := d.Purge(context.TODO(), dr)
	require.Nil(t, err)
}

func TestDeployer_Exists(t *testing.T) {
	dr := types.DeployRequest{
		SpaceID:  0,
		DeployID: 1,
		UserUUID: "1",
		Path:     "namespace/name",
		Type:     types.InferenceType,
	}

	t.Run("fail to check", func(t *testing.T) {
		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().Exist(mock.Anything, mock.Anything).
			Return(&types.StatusResponse{
				Code: -1,
			}, nil)

		d := &deployer{
			imageRunner: mockRunner,
		}
		resp, err := d.Exist(context.TODO(), dr)
		require.NotNil(t, err)
		require.True(t, resp)
	})
	t.Run("success to check", func(t *testing.T) {
		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().Exist(mock.Anything, mock.Anything).
			Return(&types.StatusResponse{
				Code: 1,
			}, nil)

		d := &deployer{
			imageRunner: mockRunner,
		}
		resp, err := d.Exist(context.TODO(), dr)
		require.Nil(t, err)
		require.True(t, resp)
	})

	t.Run("service not exist", func(t *testing.T) {
		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().Exist(mock.Anything, mock.Anything).
			Return(&types.StatusResponse{
				Code: common.Stopped,
			}, nil)

		d := &deployer{
			imageRunner: mockRunner,
		}
		resp, err := d.Exist(context.TODO(), dr)
		require.Nil(t, err)
		require.False(t, resp)
	})
}

func TestDeployer_GetReplica(t *testing.T) {
	dr := types.DeployRequest{
		SpaceID:  0,
		DeployID: 1,
		UserUUID: "1",
		Path:     "namespace/name",
		Type:     types.InferenceType,
	}

	t.Run("fail to check", func(t *testing.T) {
		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().GetReplica(mock.Anything, mock.Anything).
			Return(nil, errors.New("error"))

		d := &deployer{
			imageRunner: mockRunner,
		}
		actualReplica, desiredReplica, instances, err := d.GetReplica(context.TODO(), dr)
		require.NotNil(t, err)
		require.Equal(t, 0, actualReplica)
		require.Equal(t, 0, desiredReplica)
		require.Equal(t, 0, len(instances))
	})

	t.Run("success", func(t *testing.T) {
		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().GetReplica(mock.Anything, mock.Anything).
			Return(&types.ReplicaResponse{
				DeployID:       1,
				Code:           1,
				Message:        "",
				ActualReplica:  1,
				DesiredReplica: 1,
				Instances: []types.Instance{
					{
						Name:   "name1",
						Status: "running",
					},
				},
			}, nil)

		d := &deployer{
			imageRunner: mockRunner,
		}
		actualReplica, desiredReplica, instances, err := d.GetReplica(context.TODO(), dr)
		require.Nil(t, err)
		require.Equal(t, 1, actualReplica)
		require.Equal(t, 1, desiredReplica)
		require.Equal(t, 1, len(instances))
	})
}

func TestDeployer_InstanceLogs(t *testing.T) {
	dr := types.DeployRequest{
		SpaceID:   0,
		DeployID:  1,
		UserUUID:  "1",
		Path:      "namespace/name",
		Type:      types.InferenceType,
		ClusterID: "test",
	}

	deploy := &database.Deploy{
		Status:    common.Running,
		SvcName:   "svc",
		ClusterID: "test",
	}

	mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
	mockDeployTaskStore.EXPECT().GetDeployByID(mock.Anything, dr.DeployID).
		Return(deploy, nil)

	sender := mockSender.NewMockLogSender(t)

	ch := make(chan string)
	sender.EXPECT().StreamAllLogs(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(ch, nil)
	d := &deployer{
		deployTaskStore: mockDeployTaskStore,
		lokiClient:      sender,
	}
	lreader, err := d.InstanceLogs(context.TODO(), dr)
	require.Nil(t, err)
	require.Nil(t, lreader.buildLogs)
	require.NotNil(t, lreader.RunLog())
}

func TestDeployer_ListCluster(t *testing.T) {

	clusterResp := []types.ClusterRes{
		{
			ClusterID: "cluster1",
			Region:    "us-east-1",
			Zone:      "us-east-1a",
			Provider:  "aws",
			Enable:    false,
			Resources: []types.NodeResourceInfo{
				{

					NodeName: "node1",
					NodeHardware: types.NodeHardware{
						XPUModel:         "",
						TotalCPU:         1,
						AvailableCPU:     1,
						TotalXPU:         0,
						AvailableXPU:     0,
						GPUVendor:        "nvidia",
						TotalMem:         0,
						AvailableMem:     128,
						XPUCapacityLabel: "",
					},
				},
			},
		},
	}
	mockRunner := mockrunner.NewMockRunner(t)
	mockRunner.EXPECT().ListCluster(mock.Anything).
		Return(clusterResp, nil)

	d := &deployer{
		imageRunner: mockRunner,
	}
	clusterResources, err := d.ListCluster(context.TODO())
	require.Nil(t, err)
	require.Len(t, clusterResources, 1)
	require.Len(t, clusterResources[0].Resources, 1)
}

func TestDeployer_UpdateDeploy(t *testing.T) {
	var runtimeFrameworkID int64 = 1
	var ResourceID int64 = 1
	var deployName = "deploy1"
	var env = "{}"
	var MinReplica = 1
	var MaxReplica = 1
	var Revision = "1"
	var SecureLevel = 1
	var ClusterID = "cluster1"
	dur := &types.DeployUpdateReq{
		RuntimeFrameworkID: &runtimeFrameworkID,
		ResourceID:         &ResourceID,
		DeployName:         &deployName,
		Env:                &env,
		MinReplica:         &MinReplica,
		MaxReplica:         &MaxReplica,
		Revision:           &Revision,
		SecureLevel:        &SecureLevel,
		ClusterID:          &ClusterID,
	}

	dbdeploy := &database.Deploy{}

	mockRTFM := mockdb.NewMockRuntimeFrameworksStore(t)
	mockRTFM.EXPECT().FindEnabledByID(mock.Anything, runtimeFrameworkID).
		Return(&database.RuntimeFramework{
			FrameImage:    "gpu_image",
			FrameName:     "gpu",
			ContainerPort: 8000,
		}, nil)
	mockSpaceResourceStore := mockdb.NewMockSpaceResourceStore(t)
	mockSpaceResourceStore.EXPECT().FindByID(mock.Anything, ResourceID).
		Return(&database.SpaceResource{
			ID:        ResourceID,
			Resources: `{ "gpu": { "type": "A10", "num": "1", "resource_name": "nvidia.com/gpu", "labels": { "aliyun.accelerator/nvidia_name": "NVIDIA-A10" } }, "cpu": { "type": "Intel", "num": "12" },  "memory": "46Gi" }`,
		}, nil)

	mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
	mockDeployTaskStore.EXPECT().UpdateDeploy(mock.Anything, mock.Anything).Return(nil)
	d := &deployer{
		runtimeFrameworkStore: mockRTFM,
		deployTaskStore:       mockDeployTaskStore,
		spaceResourceStore:    mockSpaceResourceStore,
	}
	err := d.UpdateDeploy(context.TODO(), dur, dbdeploy)
	require.Nil(t, err)
}

func TestDeployer_CheckResource(t *testing.T) {
	config := &config.Config{}
	config.Runner.VGPUResourceReqKey = "nvidia.com/vgpu"
	config.Runner.VGPUMemoryReqKey = "nvidia.com/vgpumem"

	cases := []struct {
		hardware  *types.HardWare
		available bool
	}{
		{&types.HardWare{}, true},
		{&types.HardWare{
			Gpu: types.Processor{Num: "1", Type: "t1"},
			Cpu: types.CPU{Num: "2"},
		}, true},
		{&types.HardWare{
			Gpu: types.Processor{Num: "1", Type: "t2"},
			Cpu: types.CPU{Num: "2"},
		}, false},
		{&types.HardWare{
			Gpu: types.Processor{Num: "15", Type: "t1"},
			Cpu: types.CPU{Num: "2"},
		}, false},
		{&types.HardWare{
			Gpu: types.Processor{Num: "1", Type: "t1"},
			Cpu: types.CPU{Num: "20"},
		}, false},
		{&types.HardWare{
			Gpu: types.Processor{Num: "1", Type: "t1"},
			Cpu: types.CPU{Num: "12"},
		}, true},
	}

	for _, c := range cases {
		c.hardware.Memory = "1Gi"
		v, _ := CheckResource(&types.ClusterRes{
			Resources: []types.NodeResourceInfo{
				{
					NodeHardware: types.NodeHardware{
						AvailableXPU: 10, XPUModel: "t1", AvailableCPU: 10, AvailableMem: 10000,
					},
				},
				{
					NodeHardware: types.NodeHardware{
						AvailableXPU: 12, XPUModel: "t1", AvailableCPU: 12, AvailableMem: 10000,
					},
				},
			},
		}, c.hardware, config)
		require.Equal(t, c.available, v, c.hardware)
	}

}

func TestDeployer_SubmitEvaluation(t *testing.T) {
	tester := newTestDeployer(t)
	ctx := context.TODO()

	tester.mocks.runner.EXPECT().SubmitWorkFlow(ctx, mock.Anything).RunAndReturn(
		func(ctx context.Context, awfr *types.ArgoWorkFlowReq) (*types.ArgoWorkFlowRes, error) {
			require.Equal(t, map[string]string{
				"REVISIONS":               "main",
				"MODEL_IDS":               "",
				"DATASET_IDS":             "",
				"USE_CUSTOM_DATASETS":     "false",
				"DATASET_REVISIONS":       "",
				"ACCESS_TOKEN":            "k",
				"HF_ENDPOINT":             "dl",
				"HF_HUB_DOWNLOAD_TIMEOUT": "30",
			}, awfr.Templates[0].Env)
			return &types.ArgoWorkFlowRes{ID: 1}, nil
		},
	)
	resp, err := tester.SubmitEvaluation(ctx, types.EvaluationReq{
		ModelId:          "m1",
		Token:            "k",
		DownloadEndpoint: "dl",
		Revisions:        []string{"main"},
	})
	require.NoError(t, err)
	require.Equal(t, &types.ArgoWorkFlowRes{ID: 1}, resp)
}

func TestDeployer_GetEvaluation(t *testing.T) {
	tester := newTestDeployer(t)
	ctx := context.TODO()

	tester.mocks.runner.EXPECT().GetWorkFlow(ctx, types.EvaluationGetReq{}).Return(
		&types.ArgoWorkFlowRes{ID: 100}, nil,
	)
	r, err := tester.GetEvaluation(ctx, types.EvaluationGetReq{})
	require.NoError(t, err)
	require.Equal(t, &types.ArgoWorkFlowRes{ID: 100}, r)
}

func TestDeployer_SubmitFinetune(t *testing.T) {
	tester := newTestDeployer(t)
	ctx := context.TODO()

	tester.mocks.runner.EXPECT().SubmitFinetuneJob(ctx, mock.Anything).RunAndReturn(
		func(ctx context.Context, awfr *types.ArgoWorkFlowReq) (*types.ArgoWorkFlowRes, error) {
			require.Equal(t, map[string]string{
				"MODEL_ID":                "m1",
				"ACCESS_TOKEN":            "k",
				"DATASET_ID":              "",
				"HF_ENDPOINT":             "dl/hf",
				"HF_HUB_DOWNLOAD_TIMEOUT": "30",
				"HF_TOKEN":                "k",
				"HF_USERNAME":             "",
				"LEARNING_RATE":           "0",
				"CUSTOM_ARGS":             "",
				"EPOCHS":                  "0",
				"REVISION":               "",
				"DATASET_REVISION":        "",
			}, awfr.Templates[0].Env)
			return &types.ArgoWorkFlowRes{ID: 1}, nil
		},
	)

	tester.mocks.stores.ClusterInfoMock().EXPECT().FindNodeByClusterID(ctx, "").Return([]database.ClusterNode{
		{
			Name: "node1",
		},
	}, nil)

	resp, err := tester.SubmitFinetuneJob(ctx, types.FinetuneReq{
		ModelId:          "m1",
		Token:            "k",
		DownloadEndpoint: "dl",
	})
	require.NoError(t, err)
	require.Equal(t, &types.ArgoWorkFlowRes{ID: 1}, resp)
}

func TestDeployer_DeleteFinetune(t *testing.T) {
	tester := newTestDeployer(t)
	ctx := context.TODO()

	tester.mocks.runner.EXPECT().DeleteWorkFlow(ctx, types.ArgoWorkFlowDeleteReq{
		ID: 1,
	}).Return(nil, nil)

	err := tester.DeleteFinetuneJob(ctx, types.ArgoWorkFlowDeleteReq{
		ID: 1,
	})
	require.NoError(t, err)
}

func TestDeployer_DeleteEvaluation(t *testing.T) {
	tester := newTestDeployer(t)
	ctx := context.TODO()

	tester.mocks.runner.EXPECT().DeleteWorkFlow(ctx, types.ArgoWorkFlowDeleteReq{
		ID: 1,
	}).Return(nil, nil)

	err := tester.DeleteEvaluation(ctx, types.ArgoWorkFlowDeleteReq{
		ID: 1,
	})
	require.NoError(t, err)
}

func TestDeployer_DeleteEvaluation_Error(t *testing.T) {
	tester := newTestDeployer(t)
	ctx := context.TODO()

	tester.mocks.runner.EXPECT().DeleteWorkFlow(ctx, types.ArgoWorkFlowDeleteReq{
		ID: 1,
	}).Return(nil, errors.New("delete workflow failed"))

	err := tester.DeleteEvaluation(ctx, types.ArgoWorkFlowDeleteReq{
		ID: 1,
	})
	require.Error(t, err)
	require.Equal(t, "delete workflow failed", err.Error())
}

func TestDeployer_GetWorkflowLogsInStream(t *testing.T) {
	now := time.Now()
	req := types.FinetuneLogReq{
		CurrentUser: "test-user",
		PodName:     "pod1",
		SubmitTime:  now,
	}

	mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)

	sender := mockSender.NewMockLogSender(t)

	ch := make(chan string)
	sender.EXPECT().StreamAllLogs(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(ch, nil)
	d := &deployer{
		deployTaskStore: mockDeployTaskStore,
		lokiClient:      sender,
	}
	lreader, err := d.GetWorkflowLogsInStream(context.TODO(), req)
	require.Nil(t, err)
	require.Nil(t, lreader.buildLogs)
	require.NotNil(t, lreader.RunLog())
}

func TestDeployer_GetWorkflowLogsNonStream(t *testing.T) {
	now := time.Now()
	req := types.FinetuneLogReq{
		CurrentUser: "test-user",
		PodName:     "pod1",
		SubmitTime:  now,
	}

	mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)

	sender := mockSender.NewMockLogSender(t)

	sender.EXPECT().GenerateLabelQuery(mock.Anything).Return("test_query")
	sender.EXPECT().QueryRange(mock.Anything, mock.Anything).Return(&loki.LokiQueryResponse{}, nil)

	d := &deployer{
		deployTaskStore: mockDeployTaskStore,
		lokiClient:      sender,
	}
	resp, err := d.GetWorkflowLogsNonStream(context.TODO(), req)
	require.Nil(t, err)
	require.NotNil(t, resp)
}

func TestDeployer_CheckHeartbeatTimeout(t *testing.T) {
	ctx := context.TODO()
	t.Run("should return true when a cluster times out", func(t *testing.T) {
		clusterStore := mockdb.NewMockClusterInfoStore(t)
		cc := &deployer{
			clusterStore: clusterStore,
		}
		clusters := []database.ClusterInfo{
			{
				ClusterID: "c1_timed_out",
				Status:    types.ClusterStatusRunning,
			},
			{
				ClusterID: "c2_active",
				Status:    types.ClusterStatusRunning,
			},
		}
		clusters[0].UpdatedAt = time.Now().Add(-10 * time.Minute)
		clusters[1].UpdatedAt = time.Now()
		clusterStore.EXPECT().ByClusterID(ctx, clusters[0].ClusterID).Once().Return(clusters[0], nil)
		clusterStore.EXPECT().ByClusterID(ctx, clusters[1].ClusterID).Once().Return(clusters[1], nil)
		cc.deployConfig.HeartBeatTimeInSec = 5 // seconds

		timedOut, err := cc.CheckHeartbeatTimeout(ctx, "c1_timed_out")
		require.NoError(t, err)
		require.True(t, timedOut)

		timedOut, err = cc.CheckHeartbeatTimeout(ctx, "c2_active")
		require.NoError(t, err)
		require.False(t, timedOut)
	})

	t.Run("should return false when no cluster times out", func(t *testing.T) {
		clusterStore := mockdb.NewMockClusterInfoStore(t)
		cc := &deployer{
			clusterStore: clusterStore,
		}
		clusters := []database.ClusterInfo{
			{
				ClusterID: "c1_active",
				Status:    types.ClusterStatusRunning,
			},
			{
				ClusterID: "c2_active",
				Status:    types.ClusterStatusRunning,
			},
			{
				ClusterID: "c3_unavailable",
				Status:    types.ClusterStatusUnavailable,
			},
		}

		clusters[0].UpdatedAt = time.Now().Add(-1 * time.Minute)
		clusters[1].UpdatedAt = time.Now()
		clusters[2].UpdatedAt = time.Now().Add(-20 * time.Minute) // Should be timeout

		clusterStore.EXPECT().ByClusterID(ctx, clusters[0].ClusterID).Once().Return(clusters[0], nil)
		clusterStore.EXPECT().ByClusterID(ctx, clusters[1].ClusterID).Once().Return(clusters[1], nil)
		clusterStore.EXPECT().ByClusterID(ctx, clusters[2].ClusterID).Once().Return(clusters[2], nil)
		cc.deployConfig.HeartBeatTimeInSec = 5 * 60 // seconds
		timedOut, err := cc.CheckHeartbeatTimeout(ctx, "c1_active")
		require.NoError(t, err)
		require.False(t, timedOut)

		timedOut, err = cc.CheckHeartbeatTimeout(ctx, "c2_active")
		require.NoError(t, err)
		require.False(t, timedOut)

		timedOut, err = cc.CheckHeartbeatTimeout(ctx, "c3_unavailable")
		require.NoError(t, err)
		require.True(t, timedOut)
	})
}

func TestDeployer_IsDefaultScheduler(t *testing.T) {
	t.Run("default scheduler", func(t *testing.T) {
		d := &deployer{
			kubeScheduler: nil,
		}
		require.True(t, d.IsDefaultScheduler())
	})

	t.Run("custom scheduler", func(t *testing.T) {
		d := &deployer{
			kubeScheduler: &types.Scheduler{},
		}
		require.False(t, d.IsDefaultScheduler())
	})
}

func TestDeployer_GetSharedModeResourceName(t *testing.T) {
	config := &config.Config{}
	config.Runner.VGPUResourceReqKey = "nvidia.com/vgpu"

	t.Run("default scheduler", func(t *testing.T) {
		d := &deployer{
			kubeScheduler: nil,
		}
		name := d.GetSharedModeResourceName(config)
		require.Equal(t, common.DefaultResourceName, name)
	})

	t.Run("custom scheduler", func(t *testing.T) {
		d := &deployer{
			kubeScheduler: &types.Scheduler{},
		}
		name := d.GetSharedModeResourceName(config)
		require.Equal(t, "nvidia.com/vgpu", name)
	})
}

func TestDeployer_Wakeup(t *testing.T) {
	ctx := context.Background()

	t.Run("cluster with app endpoint uses app endpoint", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		mockClusterStore := mockdb.NewMockClusterInfoStore(t)
		mockClusterStore.EXPECT().ByClusterID(ctx, "cluster-1").Return(database.ClusterInfo{
			ClusterID:   "cluster-1",
			AppEndpoint: ts.URL,
		}, nil)

		d := &deployer{
			internalRootDomain: "svc.cluster.local",
			clusterStore:       mockClusterStore,
		}

		dr := types.DeployRequest{
			SpaceID:   1,
			Namespace: "test",
			Name:      "space",
			SvcName:   "test-svc",
			ClusterID: "cluster-1",
			Endpoint:  "http://test-svc.app.opencsg.com",
		}

		err := d.Wakeup(ctx, dr)
		require.NoError(t, err)
	})

	t.Run("failed to get cluster returns error", func(t *testing.T) {
		mockClusterStore := mockdb.NewMockClusterInfoStore(t)
		mockClusterStore.EXPECT().ByClusterID(ctx, "cluster-1").Return(database.ClusterInfo{}, errors.New("cluster not found"))

		d := &deployer{
			internalRootDomain: "svc.cluster.local",
			clusterStore:       mockClusterStore,
		}

		dr := types.DeployRequest{
			SpaceID:   1,
			Namespace: "test",
			Name:      "space",
			SvcName:   "test-svc",
			ClusterID: "cluster-1",
		}

		err := d.Wakeup(ctx, dr)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cluster not found")
	})

	t.Run("http request timeout returns nil", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		mockClusterStore := mockdb.NewMockClusterInfoStore(t)
		mockClusterStore.EXPECT().ByClusterID(ctx, "cluster-1").Return(database.ClusterInfo{
			ClusterID:   "cluster-1",
			AppEndpoint: ts.URL,
		}, nil)

		d := &deployer{
			internalRootDomain: "svc.cluster.local",
			clusterStore:       mockClusterStore,
		}

		dr := types.DeployRequest{
			SpaceID:   1,
			Namespace: "test",
			Name:      "space",
			SvcName:   "test-svc",
			ClusterID: "cluster-1",
			Endpoint:  "http://test-svc.app.opencsg.com",
		}

		err := d.Wakeup(ctx, dr)
		require.NoError(t, err)
	})

	t.Run("http request error is handled asynchronously", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		}))
		defer ts.Close()

		mockClusterStore := mockdb.NewMockClusterInfoStore(t)
		mockClusterStore.EXPECT().ByClusterID(ctx, "cluster-1").Return(database.ClusterInfo{
			ClusterID:   "cluster-1",
			AppEndpoint: ts.URL,
		}, nil)

		d := &deployer{
			internalRootDomain: "svc.cluster.local",
			clusterStore:       mockClusterStore,
		}

		dr := types.DeployRequest{
			SpaceID:   1,
			Namespace: "test",
			Name:      "space",
			SvcName:   "test-svc",
			ClusterID: "cluster-1",
			Endpoint:  "http://test-svc.app.opencsg.com",
		}

		err := d.Wakeup(ctx, dr)
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("http status not found is handled asynchronously", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		mockClusterStore := mockdb.NewMockClusterInfoStore(t)
		mockClusterStore.EXPECT().ByClusterID(ctx, "cluster-1").Return(database.ClusterInfo{
			ClusterID:   "cluster-1",
			AppEndpoint: ts.URL,
		}, nil)

		d := &deployer{
			internalRootDomain: "svc.cluster.local",
			clusterStore:       mockClusterStore,
		}

		dr := types.DeployRequest{
			SpaceID:   1,
			Namespace: "test",
			Name:      "space",
			SvcName:   "test-svc",
			ClusterID: "cluster-1",
			Endpoint:  "http://test-svc.app.opencsg.com",
		}

		err := d.Wakeup(ctx, dr)
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
	})
}
