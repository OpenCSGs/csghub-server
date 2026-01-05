package component

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
	knativefake "knative.dev/serving/pkg/client/clientset/versioned/fake"
	mockReporter "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/reporter"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	argofake "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/fake"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockCluster "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/cluster"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestArgoComponent_CreateWorkflow(t *testing.T) {
	argoStore := mockdb.NewMockArgoWorkFlowStore(t)
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	expectCluster := &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
		ArgoClient:    argofake.NewSimpleClientset(),
	}

	reporter := mockReporter.NewMockLogCollector(t)
	wfc := workFlowComponentImpl{
		wf:          argoStore,
		clusterPool: pool,
		config:      &config.Config{},
		logReporter: reporter,
	}
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(expectCluster, nil)
	ctx := context.TODO()
	argoStore.EXPECT().CreateWorkFlow(ctx, mock.Anything).Return(&database.ArgoWorkflow{
		ID:        1,
		ClusterID: "test",
		RepoType:  "test",
		TaskName:  "test",
		TaskId:    "test",
		Username:  "test",
		UserUUID:  "test",
		RepoIds:   []string{"test"},
		Status:    v1alpha1.WorkflowPhase(v1alpha1.NodePending),
	}, nil)
	reporter.EXPECT().Report(mock.Anything)

	wf, err := wfc.CreateWorkflow(ctx, types.ArgoWorkFlowReq{
		ClusterID: "test",
		RepoType:  string(types.ModelRepo),
		TaskName:  "test",
		TaskId:    "test",
		Username:  "test",
		UserUUID:  "test",
		RepoIds:   []string{"test"},
		Datasets:  []string{"test"},
		Image:     "test",
	})
	require.Nil(t, err)
	require.Equal(t, v1alpha1.WorkflowPhase(v1alpha1.NodePending), wf.Status)
}

// func TestArgoComponent_UpdateWorkflow(t *testing.T) {
// 	argoStore := mockdb.NewMockArgoWorkFlowStore(t)
// 	pool := &cluster.ClusterPool{}
// 	cis := mockdb.NewMockClusterInfoStore(t)
// 	pool.ClusterStore = cis
// 	kubeClient := fake.NewSimpleClientset()
// 	argoClient := argofake.NewSimpleClientset()
// 	pool.Clusters = append(pool.Clusters, cluster.Cluster{
// 		CID:           "config",
// 		ID:            "test",
// 		Client:        kubeClient,
// 		KnativeClient: knativefake.NewSimpleClientset(),
// 		ArgoClient:    argoClient,
// 	})
// 	wfc := workFlowComponentImpl{
// 		wf:          argoStore,
// 		clusterPool: pool,
// 		config:      &config.Config{},
// 	}
// 	cis.EXPECT().ByClusterID(mock.Anything, "test").Return(database.ClusterInfo{
// 		ClusterID:     "test",
// 		ClusterConfig: "config",
// 		StorageClass:  "test",
// 	}, nil)
// 	ctx := context.TODO()
// 	argoStore.EXPECT().CreateWorkFlow(ctx, mock.Anything).Return(&database.ArgoWorkflow{
// 		ID:        1,
// 		ClusterID: "test",
// 		RepoType:  "test",
// 		TaskName:  "test",
// 		TaskId:    "test",
// 		Username:  "test",
// 		UserUUID:  "test",
// 		RepoIds:   []string{"test"},
// 		Status:    v1alpha1.WorkflowPhase(v1alpha1.NodePending),
// 	}, nil)
// 	wf, err := wfc.CreateWorkflow(ctx, types.ArgoWorkFlowReq{
// 		ClusterID: "test",
// 		RepoType:  string(types.ModelRepo),
// 		TaskName:  "test",
// 		TaskId:    "test",
// 		Username:  "test",
// 		UserUUID:  "test",
// 		RepoIds:   []string{"test"},
// 		Datasets:  []string{"test"},
// 		Image:     "test",
// 	})
// 	require.Nil(t, err)
// 	require.Equal(t, v1alpha1.WorkflowPhase(v1alpha1.NodePending), wf.Status)
// 	oldWF, err := argoClient.ArgoprojV1alpha1().Workflows("").Get(ctx, "test", metav1.GetOptions{})
// 	require.Nil(t, err)
// 	oldWF.Status = v1alpha1.WorkflowStatus{
// 		Phase: v1alpha1.WorkflowRunning,
// 	}
// 	arf, err := wfc.UpdateWorkflow(ctx, oldWF)
// 	require.Nil(t, err)
// 	require.Equal(t, v1alpha1.WorkflowPhase(v1alpha1.WorkflowRunning), arf.Status)
// }

func TestArgoComponent_DeleteWorkflow(t *testing.T) {
	argoStore := mockdb.NewMockArgoWorkFlowStore(t)
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewSimpleClientset()
	argoClient := argofake.NewSimpleClientset()
	expectCluster := &cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        kubeClient,
		KnativeClient: knativefake.NewSimpleClientset(),
		ArgoClient:    argoClient,
	}
	wfc := workFlowComponentImpl{
		wf:          argoStore,
		clusterPool: pool,
		config:      &config.Config{},
		logReporter: mockReporter.NewMockLogCollector(t),
	}
	ctx := context.TODO()
	req := &types.ArgoWorkFlowDeleteReq{
		ID:        1,
		TaskID:    "test",
		ClusterID: "test",
		Namespace: "test",
	}
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(expectCluster, nil)
	err := wfc.DeleteWorkflow(ctx, req)
	require.Nil(t, err)

}
