//go:build !ee && !saas

package deploy

import (
	"context"
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
