package component

import (
	"context"
	"testing"

	mockReporter "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/reporter"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	argofake "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestImagebuilderComponent_Build(t *testing.T) {
	conf := &config.Config{}
	testClusterInfo := database.ClusterInfo{
		ClusterID:     "config",
		ClusterConfig: "config",
		StorageClass:  "",
		Region:        "",
		Zone:          "",
		Provider:      "",
	}
	testCluster := &cluster.Cluster{
		CID:        "config",
		ID:         "config",
		Client:     fake.NewSimpleClientset(),
		ArgoClient: argofake.NewSimpleClientset(),
	}

	cStore := mockdb.NewMockClusterInfoStore(t)
	logReporter := mockReporter.NewMockLogCollector(t)
	ibc := &imagebuilderComponentImpl{
		clusterPool: &cluster.ClusterPool{
			Clusters:     []*cluster.Cluster{testCluster},
			ClusterStore: cStore,
		},
		config:      conf,
		logReporter: logReporter,
	}

	logReporter.EXPECT().Report(mock.Anything).Return().Once()

	cStore.EXPECT().ByClusterConfig(context.Background(), testCluster.CID).Return(testClusterInfo, nil).Once()
	// imageStore.EXPECT().CreateOrUpdateByBuildID(context.Background(), mock.Anything).Return(&database.ImageBuilderWork{}, nil).Once()
	// imageStore.EXPECT().FindByImagePath(context.Background(), mock.Anything).Return(nil, nil).Once()
	err := ibc.Build(context.Background(), types.ImageBuilderRequest{
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

	ibc := &imagebuilderComponentImpl{
		clusterPool: &cluster.ClusterPool{
			Clusters: []*cluster.Cluster{testCluster},
		},
		config: conf,
	}
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
		OrgName:   "test-org",
		SpaceName: "test-space",
		DeployId:  "test-build-id",
		TaskId:    "test-task-id",
	})
	require.Nil(t, err)
}
