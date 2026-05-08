package component

import (
	"context"
	"errors"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	argofake "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	mockCluster "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/cluster"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func newTestDataflowComponent(t *testing.T) (*dataflowComponentImpl, *mockCluster.MockPool) {
	pool := mockCluster.NewMockPool(t)
	df := &dataflowComponentImpl{
		config:      &config.Config{},
		clusterPool: pool,
		namespace:   "test-ns",
		wfStore:     mockdb.NewMockArgoWorkFlowStore(t),
	}
	return df, pool
}

func TestDataflowComponent_CreateWorkflow(t *testing.T) {
	ctx := context.TODO()

	t.Run("success", func(t *testing.T) {
		df, pool := newTestDataflowComponent(t)
		kubeClient := fake.NewSimpleClientset()
		testCluster := &cluster.Cluster{
			CID:        "config",
			ID:         "test-cluster",
			Client:     kubeClient,
			ArgoClient: argofake.NewSimpleClientset(),
		}
		pool.EXPECT().GetClusterByID(ctx, "test-cluster").Return(testCluster, nil)

		req := &types.DataflowArgoJobReq{
			ID:          1,
			ClusterID:   "test-cluster",
			ArgoTaskID:  "df-test-task",
			JobID:       "df-test-job",
			JobName:     "test-job",
			JobDesc:     "test desc",
			StorageSize: "10Gi",
			Entrypoint:  "main",
			Template: types.ArgoFlowTemplate{
				Name:       "echo",
				Image:      "alpine:latest",
				Command:    []string{"echo"},
				Args:       []string{"hello"},
				Parameters: []string{"cmd", "task_id"},
			},
			DagTasks: []types.ArgoDagTask{
				{ID: "task-1", Name: "task1", Template: "echo", Deps: []string{}},
			},
			Nodes: []types.Node{
				{Name: "node-1"},
			},
		}

		resp, err := df.CreateWorkflow(ctx, req)
		require.NoError(t, err)
		require.Equal(t, req.ID, resp.ID)
		require.Equal(t, req.ArgoTaskID, resp.ArgoTaskID)
		require.Equal(t, req.JobID, resp.JobID)
		require.Equal(t, req.JobName, resp.JobName)
		require.Equal(t, "Pending", resp.Status)

		pvcName := types.DFPVCNamePrefix + req.ArgoTaskID
		pvc, err := kubeClient.CoreV1().PersistentVolumeClaims(df.namespace).Get(ctx, pvcName, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, pvcName, pvc.Name)

		wf, err := testCluster.ArgoClient.ArgoprojV1alpha1().Workflows(df.namespace).Get(ctx, req.ArgoTaskID, metav1.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, req.ArgoTaskID, wf.Name)
	})

	t.Run("cluster not found", func(t *testing.T) {
		df, pool := newTestDataflowComponent(t)
		pool.EXPECT().GetClusterByID(ctx, "unknown-cluster").Return(nil, errors.New("cluster not found"))

		req := &types.DataflowArgoJobReq{
			ClusterID: "unknown-cluster",
		}

		resp, err := df.CreateWorkflow(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "failed to get cluster")
	})

	t.Run("argo create fails and cleans up pvc", func(t *testing.T) {
		df, pool := newTestDataflowComponent(t)
		kubeClient := fake.NewSimpleClientset()
		argoClient := argofake.NewSimpleClientset()
		testCluster := &cluster.Cluster{
			CID:        "config",
			ID:         "test-cluster",
			Client:     kubeClient,
			ArgoClient: argoClient,
		}
		pool.EXPECT().GetClusterByID(ctx, "test-cluster").Return(testCluster, nil)

		req := &types.DataflowArgoJobReq{
			ID:          1,
			ClusterID:   "test-cluster",
			ArgoTaskID:  "df-test-fail",
			JobID:       "df-test-job",
			JobName:     "test-job",
			JobDesc:     "test desc",
			StorageSize: "10Gi",
			Entrypoint:  "main",
			Template: types.ArgoFlowTemplate{
				Name:       "echo",
				Image:      "alpine:latest",
				Command:    []string{"echo"},
				Args:       []string{"hello"},
				Parameters: []string{"cmd", "task_id"},
			},
			DagTasks: []types.ArgoDagTask{
				{ID: "task-1", Name: "task1", Template: "echo", Deps: []string{}},
			},
			Nodes: []types.Node{
				{Name: "node-1"},
			},
		}

		existingWF := &v1alpha1.Workflow{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: df.namespace,
				Name:      req.ArgoTaskID,
			},
		}
		_, err := argoClient.ArgoprojV1alpha1().Workflows(df.namespace).Create(ctx, existingWF, metav1.CreateOptions{})
		require.NoError(t, err)

		resp, err := df.CreateWorkflow(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "failed to create dataflow workflow")

		pvcName := types.DFPVCNamePrefix + req.ArgoTaskID
		_, err = kubeClient.CoreV1().PersistentVolumeClaims(df.namespace).Get(ctx, pvcName, metav1.GetOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})
}

func TestDataflowComponent_deletePVC(t *testing.T) {
	ctx := context.TODO()

	t.Run("success", func(t *testing.T) {
		df, _ := newTestDataflowComponent(t)
		kubeClient := fake.NewSimpleClientset()
		testCluster := &cluster.Cluster{
			Client: kubeClient,
		}
		req := &types.DataflowArgoReq{
			ArgoTaskID: "df-test-pvc",
		}

		pvcName := types.DFPVCNamePrefix + req.ArgoTaskID
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: df.namespace,
				Name:      pvcName,
			},
		}
		_, err := kubeClient.CoreV1().PersistentVolumeClaims(df.namespace).Create(ctx, pvc, metav1.CreateOptions{})
		require.NoError(t, err)

		_, err = kubeClient.CoreV1().PersistentVolumeClaims(df.namespace).Get(ctx, pvcName, metav1.GetOptions{})
		require.NoError(t, err)

		err = df.deletePVC(ctx, testCluster, req)
		require.NoError(t, err)

		_, err = kubeClient.CoreV1().PersistentVolumeClaims(df.namespace).Get(ctx, pvcName, metav1.GetOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("delete non-existent pvc returns error", func(t *testing.T) {
		df, _ := newTestDataflowComponent(t)
		kubeClient := fake.NewSimpleClientset()
		testCluster := &cluster.Cluster{
			Client: kubeClient,
		}
		req := &types.DataflowArgoReq{
			ArgoTaskID: "df-nonexistent",
		}

		err := df.deletePVC(ctx, testCluster, req)
		require.Error(t, err)
	})
}

func TestDataflowComponent_GetStatus(t *testing.T) {
	ctx := context.TODO()

	t.Run("success", func(t *testing.T) {
		df, pool := newTestDataflowComponent(t)
		argoClient := argofake.NewSimpleClientset()
		testCluster := &cluster.Cluster{
			CID:        "config",
			ID:         "test-cluster",
			ArgoClient: argoClient,
		}
		pool.EXPECT().GetClusterByID(ctx, "test-cluster").Return(testCluster, nil)

		wf := &v1alpha1.Workflow{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: df.namespace,
				Name:      "df-test-status",
			},
			Status: v1alpha1.WorkflowStatus{
				Phase: v1alpha1.WorkflowRunning,
			},
		}
		_, err := argoClient.ArgoprojV1alpha1().Workflows(df.namespace).Create(ctx, wf, metav1.CreateOptions{})
		require.NoError(t, err)

		req := &types.DataflowArgoReq{
			ArgoTaskID: "df-test-status",
			ClusterID:  "test-cluster",
		}

		resp, err := df.GetStatus(ctx, req)
		require.NoError(t, err)
		require.Equal(t, req.ArgoTaskID, resp.ArgoTaskID)
	})

	t.Run("workflow not found", func(t *testing.T) {
		df, pool := newTestDataflowComponent(t)
		argoClient := argofake.NewSimpleClientset()
		testCluster := &cluster.Cluster{
			CID:        "config",
			ID:         "test-cluster",
			ArgoClient: argoClient,
		}
		pool.EXPECT().GetClusterByID(ctx, "test-cluster").Return(testCluster, nil)

		req := &types.DataflowArgoReq{
			ArgoTaskID: "nonexistent",
			ClusterID:  "test-cluster",
		}

		resp, err := df.GetStatus(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "workflow not found")
	})

	t.Run("cluster not found", func(t *testing.T) {
		df, pool := newTestDataflowComponent(t)
		pool.EXPECT().GetClusterByID(ctx, "unknown-cluster").Return(nil, errors.New("cluster not found"))

		req := &types.DataflowArgoReq{
			ClusterID: "unknown-cluster",
		}

		resp, err := df.GetStatus(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "cluster not found")
	})
}

func TestDataflowComponent_DeleteWorkflow(t *testing.T) {
	ctx := context.TODO()

	t.Run("success", func(t *testing.T) {
		df, pool := newTestDataflowComponent(t)
		kubeClient := fake.NewSimpleClientset()
		argoClient := argofake.NewSimpleClientset()
		testCluster := &cluster.Cluster{
			CID:        "config",
			ID:         "test-cluster",
			Client:     kubeClient,
			ArgoClient: argoClient,
		}
		pool.EXPECT().GetClusterByID(ctx, "test-cluster").Return(testCluster, nil)

		wf := &v1alpha1.Workflow{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: df.namespace,
				Name:      "df-test-delete",
			},
		}
		_, err := argoClient.ArgoprojV1alpha1().Workflows(df.namespace).Create(ctx, wf, metav1.CreateOptions{})
		require.NoError(t, err)

		pvcName := types.DFPVCNamePrefix + "df-test-delete"
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: df.namespace,
				Name:      pvcName,
			},
		}
		_, err = kubeClient.CoreV1().PersistentVolumeClaims(df.namespace).Create(ctx, pvc, metav1.CreateOptions{})
		require.NoError(t, err)

		req := &types.DataflowArgoReq{
			ArgoTaskID: "df-test-delete",
			ClusterID:  "test-cluster",
		}

		err = df.DeleteWorkflow(ctx, req)
		require.NoError(t, err)

		_, err = argoClient.ArgoprojV1alpha1().Workflows(df.namespace).Get(ctx, req.ArgoTaskID, metav1.GetOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")

		_, err = kubeClient.CoreV1().PersistentVolumeClaims(df.namespace).Get(ctx, pvcName, metav1.GetOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("workflow not found handled gracefully", func(t *testing.T) {
		df, pool := newTestDataflowComponent(t)
		kubeClient := fake.NewSimpleClientset()
		argoClient := argofake.NewSimpleClientset()
		testCluster := &cluster.Cluster{
			CID:        "config",
			ID:         "test-cluster",
			Client:     kubeClient,
			ArgoClient: argoClient,
		}
		pool.EXPECT().GetClusterByID(ctx, "test-cluster").Return(testCluster, nil)

		req := &types.DataflowArgoReq{
			ArgoTaskID: "nonexistent",
			ClusterID:  "test-cluster",
		}

		err := df.DeleteWorkflow(ctx, req)
		require.NoError(t, err)
	})

	t.Run("cluster not found", func(t *testing.T) {
		df, pool := newTestDataflowComponent(t)
		pool.EXPECT().GetClusterByID(ctx, "unknown-cluster").Return(nil, errors.New("cluster not found"))

		req := &types.DataflowArgoReq{
			ClusterID: "unknown-cluster",
		}

		err := df.DeleteWorkflow(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get cluster")
	})
}
