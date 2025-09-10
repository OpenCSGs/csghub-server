//go:build !ee && !saas

package deploy

import (
	"context"
	"errors"
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
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func newTestDeployer(t *testing.T) *testDepolyerWithMocks {
	mockStores := tests.NewMockStores(t)
	node, err := snowflake.NewNode(1)
	require.NoError(t, err)
	s := &testDepolyerWithMocks{
		deployer: &deployer{
			deployTaskStore:       mockStores.DeployTask,
			spaceStore:            mockStores.Space,
			spaceResourceStore:    mockStores.SpaceResource,
			runtimeFrameworkStore: mockStores.RuntimeFramework,
			userStore:             mockStores.User,
			snowflakeNode:         node,
		},
	}
	s.mocks.stores = mockStores
	s.mocks.scheduler = mockScheduler.NewMockScheduler(t)
	s.scheduler = s.mocks.scheduler
	s.mocks.builder = mockbuilder.NewMockBuilder(t)
	s.imageBuilder = s.mocks.builder
	s.mocks.runner = mockrunner.NewMockRunner(t)
	s.imageRunner = s.mocks.runner
	return s
}

func TestDeployer_Stop(t *testing.T) {
	dr := types.DeployRepo{
		SpaceID:  0,
		DeployID: 1,
		UserUUID: "1",
		Path:     "namespace/name",
		Type:     types.InferenceType,
	}

	mockRunner := mockrunner.NewMockRunner(t)
	mockRunner.EXPECT().Stop(mock.Anything, mock.Anything).Return(&types.StopResponse{}, nil)

	d := &deployer{
		imageRunner: mockRunner,
	}
	err := d.Stop(context.TODO(), dr)
	require.Nil(t, err)
}

func TestDeployer_StartDeploy(t *testing.T) {
	dbdeploy := database.Deploy{
		ID:       1,
		UserUUID: "1",
	}

	mockTaskStore := mockdb.NewMockDeployTaskStore(t)
	//make a copy to compare the status
	dbdeployUpdate := dbdeploy
	dbdeployUpdate.Status = common.Pending
	mockTaskStore.EXPECT().UpdateDeploy(mock.Anything, &dbdeployUpdate).Return(nil)

	buildTask := database.DeployTask{
		DeployID: dbdeploy.ID,
		TaskType: 1,
	}
	mockTaskStore.EXPECT().CreateDeployTask(mock.Anything, &buildTask).Return(nil)

	mockSch := mockScheduler.NewMockScheduler(t)
	mockSch.EXPECT().Queue(mock.Anything).Return(nil)

	node, _ := snowflake.NewNode(1)

	d := &deployer{
		snowflakeNode:   node,
		deployTaskStore: mockTaskStore,
		scheduler:       mockSch,
	}
	err := d.StartDeploy(context.TODO(), &dbdeploy)

	//wait for scheduler to queue task
	time.Sleep(time.Second)

	require.Nil(t, err)
}

func TestDeployer_CheckResourceAvailable(t *testing.T) {
	tester := newTestDeployer(t)
	ctx := context.TODO()

	tester.mocks.runner.EXPECT().ListCluster(ctx).Return([]types.ClusterResponse{
		{ClusterID: "c1"},
	}, nil)
	tester.mocks.runner.EXPECT().GetClusterById(ctx, "c1").Return(&types.ClusterResponse{
		Nodes: map[string]types.NodeResourceInfo{
			"n1": {AvailableMem: 100},
		},
	}, nil)

	v, err := tester.CheckResourceAvailable(ctx, "", 0, &types.HardWare{Memory: "10Gi"})
	require.NoError(t, err)
	require.True(t, v)
}

func TestDeployer_updateEvaluationEnvHardware(t *testing.T) {

	cases := []struct {
		hardware types.HardWare
		key      string
		value    string
	}{
		{types.HardWare{
			Gpu: types.Processor{Num: "1"},
		}, "GPU_NUM", "1"},
	}

	for _, c := range cases {
		m := map[string]string{}
		common.UpdateEvaluationEnvHardware(m, c.hardware)
		require.Equal(t, c.value, m[c.key])
	}

}

func Test_CheckNodeResource(t *testing.T) {
	baseNode := types.NodeResourceInfo{
		AvailableCPU: 16,
		AvailableMem: 8, // 8 GiB
		AvailableXPU: 2,
		XPUModel:     "NVIDIA-A100",
	}

	testCases := []struct {
		name     string
		node     types.NodeResourceInfo
		hardware *types.HardWare
		want     bool
	}{
		{
			name: "Success - All resources sufficient, including storage",
			node: baseNode,
			hardware: &types.HardWare{
				Cpu:    types.CPU{Num: "8"},
				Memory: "4Gi",
				Gpu:    types.Processor{Num: "1", Type: "NVIDIA-A100"},
			},
			want: true,
		},
		{
			name: "Success for millivalue - All resources sufficient, including storage",
			node: baseNode,
			hardware: &types.HardWare{
				Cpu:    types.CPU{Num: "800m"},
				Memory: "4Gi",
				Gpu:    types.Processor{Num: "1", Type: "NVIDIA-A100"},
			},
			want: true,
		},
		{
			name: "Failure - Insufficient Memory",
			node: baseNode,
			hardware: &types.HardWare{
				Memory: "10Gi",
			},
			want: false,
		},
		{
			name: "Failure - Mismatched XPU Type",
			node: baseNode,
			hardware: &types.HardWare{
				Gpu: types.Processor{Num: "1", Type: "NVIDIA-V100"},
			},
			want: false,
		},
		{
			name: "Failure - Invalid memory format",
			node: baseNode,
			hardware: &types.HardWare{
				Memory: "lots-of-memory",
			},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkNodeResource(tc.node, tc.hardware)
			if got != tc.want {
				t.Errorf("checkNodeResource() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDeployer_GetClusterById(t *testing.T) {
	tester := newTestDeployer(t)
	t.Run("success", func(t *testing.T) {
		ctx := context.TODO()
		tester.mocks.runner.EXPECT().GetClusterById(ctx, "1").Once().Return(&types.ClusterResponse{
			ClusterID: "1",
			Region:    "test-region",
			Zone:      "test-zone",
			Enable:    true,
			Nodes: map[string]types.NodeResourceInfo{
				"1": {
					AvailableCPU: 1,
					AvailableMem: 3,
				},
				"2": {
					AvailableCPU: 2,
					AvailableMem: 5,
					AvailableXPU: 4,
				},
			},
		}, nil)
		clusterRes, err := tester.GetClusterById(ctx, "1")
		require.Nil(t, err)
		require.Equal(t, float64(3), clusterRes.AvailableCPU)
		require.Equal(t, float64(8), clusterRes.AvailableMem)
		require.Equal(t, int64(4), clusterRes.AvailableGPU)
	})
	t.Run("empty nodes", func(t *testing.T) {
		ctx := context.TODO()
		tester.mocks.runner.EXPECT().GetClusterById(ctx, "1").Once().Return(&types.ClusterResponse{
			ClusterID: "1",
			Nodes:     nil,
		}, nil)
		clusterRes, err := tester.GetClusterById(ctx, "1")
		require.Nil(t, err)
		require.Equal(t, types.ClusterStatusRunning, clusterRes.Status)
	})
	t.Run("get cluster failed", func(t *testing.T) {
		ctx := context.TODO()
		tester.mocks.runner.EXPECT().GetClusterById(ctx, "1").Once().Return(nil, errors.New("some error"))
		clusterRes, err := tester.GetClusterById(ctx, "1")
		require.Nil(t, err)
		require.Equal(t, types.ClusterStatusUnavailable, clusterRes.Status)
	})
}
