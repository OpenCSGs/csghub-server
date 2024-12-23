package deploy

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockbuilder "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/imagebuilder"
	mockrunner "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/imagerunner"
	mockScheduler "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/scheduler"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

type testDepolyerWithMocks struct {
	*deployer
	mocks struct {
		stores    *tests.MockStores
		scheduler *mockScheduler.MockScheduler
		builder   *mockbuilder.MockBuilder
		runner    *mockrunner.MockRunner
	}
}

func TestDeployer_GenerateUniqueSvcName(t *testing.T) {
	dr := types.DeployRepo{
		Path: "namespace/name",
	}

	node, _ := snowflake.NewNode(1)
	d := &deployer{
		snowflakeNode: node,
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

		dr := types.DeployRepo{
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

		dr := types.DeployRepo{
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
	dr := types.DeployRepo{
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

	t.Run("use on-demand resource and skip build task", func(t *testing.T) {
		dr := types.DeployRepo{
			UserUUID: "1",
			Path:     "namespace/name",
			Type:     types.InferenceType,
			ImageID:  "image:1",
		}

		buildTask := database.DeployTask{
			TaskType: 0,
			Status:   scheduler.BuildSkip,
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

		mockSch := mockScheduler.NewMockScheduler(t)
		mockSch.EXPECT().Queue(mock.Anything).Return(nil)

		node, _ := snowflake.NewNode(1)

		d := &deployer{
			snowflakeNode:   node,
			deployTaskStore: mockTaskStore,
			scheduler:       mockSch,
		}

		_, err := d.Deploy(context.TODO(), dr)
		// wait for scheduler.Queue to be called
		time.Sleep(time.Second)
		require.Nil(t, err)
	})
}

func TestDeployer_Status(t *testing.T) {
	t.Run("no deploy", func(t *testing.T) {
		dr := types.DeployRepo{
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
		dr := types.DeployRepo{
			DeployID: 1,
			UserUUID: "1",
			Path:     "namespace/name",
			Type:     types.InferenceType,
		}
		deploy := &database.Deploy{
			Status:  common.Running,
			SvcName: "svc",
		}

		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockDeployTaskStore.EXPECT().GetDeployByID(mock.Anything, dr.DeployID).
			Return(deploy, nil)

		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
		}
		d.runnerStatusCache = make(map[string]types.StatusResponse)

		svcName, deployStatus, instances, err := d.Status(context.TODO(), dr, false)
		require.Nil(t, err)
		require.Equal(t, "svc", svcName)
		require.Equal(t, common.Stopped, deployStatus)
		require.Nil(t, instances)

	})

	t.Run("cache miss and not running", func(t *testing.T) {
		dr := types.DeployRepo{
			DeployID: 1,
			UserUUID: "1",
			Path:     "namespace/name",
			Type:     types.InferenceType,
		}
		deploy := &database.Deploy{
			Status:  common.BuildSuccess,
			SvcName: "svc",
		}

		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockDeployTaskStore.EXPECT().GetDeployByID(mock.Anything, dr.DeployID).
			Return(deploy, nil)

		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
		}
		d.runnerStatusCache = make(map[string]types.StatusResponse)

		svcName, deployStatus, instances, err := d.Status(context.TODO(), dr, false)
		require.Nil(t, err)
		require.Equal(t, "svc", svcName)
		require.Equal(t, common.BuildSuccess, deployStatus)
		require.Nil(t, instances)

	})

	t.Run("cache hit and running", func(t *testing.T) {
		dr := types.DeployRepo{
			DeployID: 1,
			UserUUID: "1",
			Path:     "namespace/name",
			Type:     types.InferenceType,
			ModelID:  1,
		}
		// build success status in db
		deploy := &database.Deploy{
			Status:  common.BuildSuccess,
			SvcName: "svc",
		}

		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockDeployTaskStore.EXPECT().GetDeployByID(mock.Anything, dr.DeployID).
			Return(deploy, nil)

		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().Status(mock.Anything, mock.Anything).
			Return(&types.StatusResponse{
				DeployID: 1,
				UserID:   "",
				// running status from the runner (latest)
				Code:     int(common.Running),
				Message:  "",
				Endpoint: "http://localhost",
				Instances: []types.Instance{{
					Name:   "instance1",
					Status: "ready",
				}},
				Replica:     1,
				DeployType:  0,
				ServiceName: "svc",
				DeploySku:   "",
			}, nil)

		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
			imageRunner:     mockRunner,
		}
		d.runnerStatusCache = make(map[string]types.StatusResponse)
		// deploying status in cache
		d.runnerStatusCache["svc"] = types.StatusResponse{
			DeployID: 1,
			UserID:   "",
			Code:     int(common.Deploying),
			Message:  "",
		}

		svcName, deployStatus, instances, err := d.Status(context.TODO(), dr, false)
		require.Nil(t, err)
		require.Equal(t, "svc", svcName)
		require.Equal(t, common.Running, deployStatus)
		require.Len(t, instances, 1)

	})
}

func TestDeployer_Logs(t *testing.T) {
	t.Run("no deploy", func(t *testing.T) {
		dr := types.DeployRepo{
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
		dr := types.DeployRepo{
			SpaceID:  1,
			DeployID: 1,
			UserUUID: "1",
			Path:     "namespace/name",
			Type:     types.InferenceType,
		}
		deploy := &database.Deploy{
			Status:  common.Running,
			SvcName: "svc",
		}

		mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
		mockDeployTaskStore.EXPECT().GetLatestDeployBySpaceID(mock.Anything, dr.SpaceID).
			Return(deploy, nil)

		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().Logs(mock.Anything, mock.Anything).Return(nil, nil)
		mockBuilder := mockbuilder.NewMockBuilder(t)
		mockBuilder.EXPECT().Logs(mock.Anything, mock.Anything).Return(nil, nil)
		d := &deployer{
			deployTaskStore: mockDeployTaskStore,
			imageRunner:     mockRunner,
			imageBuilder:    mockBuilder,
		}

		lreader, err := d.Logs(context.TODO(), dr)
		require.Nil(t, err)
		require.NotNil(t, lreader)

	})
}

func TestDeployer_Purge(t *testing.T) {
	dr := types.DeployRepo{
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
	dr := types.DeployRepo{
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
				Code: 2,
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
	dr := types.DeployRepo{
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
	dr := types.DeployRepo{
		SpaceID:  0,
		DeployID: 1,
		UserUUID: "1",
		Path:     "namespace/name",
		Type:     types.InferenceType,
	}

	mockRunner := mockrunner.NewMockRunner(t)
	runLog := make(chan string)
	defer close(runLog)
	mockRunner.EXPECT().InstanceLogs(mock.Anything, mock.Anything).
		Return(runLog, nil)

	d := &deployer{
		imageRunner: mockRunner,
	}
	lreader, err := d.InstanceLogs(context.TODO(), dr)
	require.Nil(t, err)
	require.Nil(t, lreader.buildLogs)
	require.NotNil(t, lreader.RunLog())
}

func TestDeployer_ListCluster(t *testing.T) {

	clusterResp := []types.ClusterResponse{
		{
			ClusterID: "cluster1",
			Region:    "us-east-1",
			Zone:      "us-east-1a",
			Provider:  "aws",
			Enable:    false,
			Nodes: map[string]types.NodeResourceInfo{
				"node1": {
					NodeName:         "node1",
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

func TestDeployer_CheckResource(t *testing.T) {

	cases := []struct {
		hardware  *types.HardWare
		available bool
	}{
		{&types.HardWare{}, true},
		{&types.HardWare{
			Gpu: types.GPU{Num: "1", Type: "t1"},
			Cpu: types.CPU{Num: "2"},
		}, true},
		{&types.HardWare{
			Gpu: types.GPU{Num: "1", Type: "t2"},
			Cpu: types.CPU{Num: "2"},
		}, false},
		{&types.HardWare{
			Gpu: types.GPU{Num: "15", Type: "t1"},
			Cpu: types.CPU{Num: "2"},
		}, false},
		{&types.HardWare{
			Gpu: types.GPU{Num: "1", Type: "t1"},
			Cpu: types.CPU{Num: "20"},
		}, false},
	}

	for _, c := range cases {
		c.hardware.Memory = "1Gi"
		v := CheckResource(&types.ClusterRes{
			Resources: []types.NodeResourceInfo{
				{AvailableXPU: 10, XPUModel: "t1", AvailableCPU: 10, AvailableMem: 10000},
			},
		}, c.hardware)
		require.Equal(t, c.available, v, c.hardware)
	}

}

func TestDeployer_SubmitEvaluation(t *testing.T) {
	tester := newTestDeployer(t)
	ctx := context.TODO()

	tester.mocks.runner.EXPECT().SubmitWorkFlow(ctx, mock.Anything).RunAndReturn(
		func(ctx context.Context, awfr *types.ArgoWorkFlowReq) (*types.ArgoWorkFlowRes, error) {
			require.Equal(t, map[string]string{
				"REVISION":     "main",
				"MODEL_ID":     "m1",
				"DATASET_IDS":  "",
				"ACCESS_TOKEN": "k",
				"HF_ENDPOINT":  "dl",
			}, awfr.Templates[0].Env)
			return &types.ArgoWorkFlowRes{ID: 1}, nil
		},
	)
	resp, err := tester.SubmitEvaluation(ctx, types.EvaluationReq{
		ModelId:          "m1",
		Token:            "k",
		DownloadEndpoint: "dl",
	})
	require.NoError(t, err)
	require.Equal(t, &types.ArgoWorkFlowRes{ID: 1}, resp)
}

func TestDeployer_ListEvaluations(t *testing.T) {
	tester := newTestDeployer(t)
	ctx := context.TODO()

	tester.mocks.runner.EXPECT().ListWorkFlows(ctx, "user", 10, 1).Return(
		&types.ArgoWorkFlowListRes{Total: 100}, nil,
	)
	r, err := tester.ListEvaluations(ctx, "user", 10, 1)
	require.NoError(t, err)
	require.Equal(t, &types.ArgoWorkFlowListRes{Total: 100}, r)
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
