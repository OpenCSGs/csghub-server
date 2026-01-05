package component

import (
	"context"
	"testing"

	mockCluster "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/cluster"
	mockReporter "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/reporter"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	argofake "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestImagebuilderComponent_Build(t *testing.T) {
	conf := &config.Config{}
	testCluster := &cluster.Cluster{
		CID:        "config",
		ID:         "config",
		Client:     fake.NewSimpleClientset(),
		ArgoClient: argofake.NewSimpleClientset(),
	}

	logReporter := mockReporter.NewMockLogCollector(t)
	pool := mockCluster.NewMockPool(t)
	ibc := &imagebuilderComponentImpl{
		clusterPool: pool,
		config:      conf,
		logReporter: logReporter,
	}

	logReporter.EXPECT().Report(mock.Anything).Return().Once()

	pool.EXPECT().GetClusterByID(context.Background(), testCluster.CID).Return(testCluster, nil).Once()
	// imageStore.EXPECT().CreateOrUpdateByBuildID(context.Background(), mock.Anything).Return(&database.ImageBuilderWork{}, nil).Once()
	// imageStore.EXPECT().FindByImagePath(context.Background(), mock.Anything).Return(nil, nil).Once()
	err := ibc.Build(context.Background(), types.ImageBuilderRequest{
		ClusterID: "config",
		OrgName:   "test-org",
		SpaceName: "test-space",
		DeployId:  "test-build-id",
		SpaceURL:  "https://github.com/test-org/test-space",
	})

	require.Nil(t, err)
}

func TestImagebuilderComponent_Stop(t *testing.T) {
	conf := &config.Config{}
	testCluster := &cluster.Cluster{
		CID:        "config",
		ID:         "config",
		Client:     fake.NewSimpleClientset(),
		ArgoClient: argofake.NewSimpleClientset(),
	}
	pool := mockCluster.NewMockPool(t)
	ibc := &imagebuilderComponentImpl{
		clusterPool: pool,
		config:      conf,
	}
	pool.EXPECT().GetClusterByID(mock.Anything, "config").Return(testCluster, nil)
	workName := ibc.generateWorkName("test-org", "test-space", "test-build-id", "test-task-id")
	_, err := testCluster.ArgoClient.ArgoprojV1alpha1().Workflows("").Create(context.TODO(), &v1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workName,
			Namespace: "",
		},
		Status: v1alpha1.WorkflowStatus{
			Phase: v1alpha1.WorkflowRunning,
		},
	}, metav1.CreateOptions{})
	require.Nil(t, err)

	err = ibc.Stop(context.Background(), types.ImageBuildStopReq{
		ClusterID: "config",
		OrgName:   "test-org",
		SpaceName: "test-space",
		DeployId:  "test-build-id",
		TaskId:    "test-task-id",
	})
	require.Nil(t, err)
}
