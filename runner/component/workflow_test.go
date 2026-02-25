package component

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestArgoComponent_UpdateWorkflow(t *testing.T) {
	argoStore := mockdb.NewMockArgoWorkFlowStore(t)
	kubeClient := fake.NewSimpleClientset()
	reporter := mockReporter.NewMockLogCollector(t)

	wfc := workFlowComponentImpl{
		wf:          argoStore,
		config:      &config.Config{},
		logReporter: reporter,
	}

	ctx := context.TODO()

	t.Run("successfully update workflow status to running", func(t *testing.T) {
		cluster := &cluster.Cluster{
			CID:           "test-cluster",
			ID:            "test",
			Client:        kubeClient,
			KnativeClient: knativefake.NewSimpleClientset(),
			ArgoClient:    argofake.NewSimpleClientset(),
		}

		existingWf := database.ArgoWorkflow{
			ID:           1,
			TaskId:       "test-task",
			Username:     "test-user",
			UserUUID:     "test-uuid",
			TaskName:     "test-task",
			TaskType:     types.TaskTypeEvaluation,
			ClusterID:    "test",
			Namespace:    "default",
			Status:       v1alpha1.WorkflowPending,
			Reason:       "",
			Image:        "test-image",
			RepoIds:      []string{"repo1"},
			RepoType:     "model",
			TaskDesc:     "test desc",
			ResourceId:   100,
			ResourceName: "test-resource",
			Datasets:     []string{"dataset1"},
		}

		updateWf := &v1alpha1.Workflow{
			ObjectMeta: v1.ObjectMeta{
				Name:      "test-task",
				Namespace: "default",
			},
			Status: v1alpha1.WorkflowStatus{
				Nodes: map[string]v1alpha1.NodeStatus{
					"test-task": {
						Phase:   v1alpha1.NodeRunning,
						Message: "workflow is running",
					},
				},
			},
		}

		argoStore.EXPECT().FindByTaskID(ctx, "test-task").Return(existingWf, nil)
		argoStore.EXPECT().UpdateWorkFlow(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			return wf.TaskId == "test-task" &&
				wf.Status == v1alpha1.WorkflowPhase(v1alpha1.NodeRunning)
		})).RunAndReturn(func(_ context.Context, wf database.ArgoWorkflow) (*database.ArgoWorkflow, error) {
			return &wf, nil
		})
		reporter.EXPECT().Report(mock.Anything)

		result, err := wfc.UpdateWorkflow(ctx, updateWf, cluster)
		require.Nil(t, err)
		require.Equal(t, int64(1), result.ID)
		require.Equal(t, v1alpha1.WorkflowPhase(v1alpha1.NodeRunning), result.Status)
	})

	t.Run("successfully update workflow status to succeeded", func(t *testing.T) {
		cluster := &cluster.Cluster{
			CID:           "test-cluster",
			ID:            "test",
			Client:        kubeClient,
			KnativeClient: knativefake.NewSimpleClientset(),
			ArgoClient:    argofake.NewSimpleClientset(),
		}

		existingWf := database.ArgoWorkflow{
			ID:           1,
			TaskId:       "test-task",
			Username:     "test-user",
			UserUUID:     "test-uuid",
			TaskName:     "test-task",
			TaskType:     types.TaskTypeEvaluation,
			ClusterID:    "test",
			Namespace:    "default",
			Status:       v1alpha1.WorkflowRunning,
			Reason:       "",
			Image:        "test-image",
			RepoIds:      []string{"repo1"},
			RepoType:     "model",
			TaskDesc:     "test desc",
			ResourceId:   100,
			ResourceName: "test-resource",
			Datasets:     []string{"dataset1"},
		}

		updateWf := &v1alpha1.Workflow{
			ObjectMeta: v1.ObjectMeta{
				Name:      "test-task",
				Namespace: "default",
			},
			Status: v1alpha1.WorkflowStatus{
				Nodes: map[string]v1alpha1.NodeStatus{
					"test-task": {
						Phase:   v1alpha1.NodeSucceeded,
						Message: "workflow succeeded",
						Outputs: &v1alpha1.Outputs{
							Parameters: []v1alpha1.Parameter{
								{
									Name:  "result",
									Value: v1alpha1.AnyStringPtr("result-url,download-url"),
								},
							},
						},
					},
				},
			},
		}

		argoStore.EXPECT().FindByTaskID(ctx, "test-task").Return(existingWf, nil)
		argoStore.EXPECT().UpdateWorkFlow(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			return wf.TaskId == "test-task" &&
				wf.Status == v1alpha1.WorkflowPhase(v1alpha1.NodeSucceeded) &&
				wf.ResultURL == "result-url" &&
				wf.DownloadURL == "download-url"
		})).RunAndReturn(func(_ context.Context, wf database.ArgoWorkflow) (*database.ArgoWorkflow, error) {
			return &wf, nil
		})
		reporter.EXPECT().Report(mock.Anything)

		result, err := wfc.UpdateWorkflow(ctx, updateWf, cluster)
		require.Nil(t, err)
		require.Equal(t, v1alpha1.WorkflowPhase(v1alpha1.NodeSucceeded), result.Status)
		require.Equal(t, "result-url", result.ResultURL)
		require.Equal(t, "download-url", result.DownloadURL)
	})

	t.Run("workflow failed - get pod logs", func(t *testing.T) {
		cluster := &cluster.Cluster{
			CID:           "test-cluster",
			ID:            "test",
			Client:        kubeClient,
			KnativeClient: knativefake.NewSimpleClientset(),
			ArgoClient:    argofake.NewSimpleClientset(),
		}

		existingWf := database.ArgoWorkflow{
			ID:           1,
			TaskId:       "test-task",
			Username:     "test-user",
			UserUUID:     "test-uuid",
			TaskName:     "test-task",
			TaskType:     types.TaskTypeEvaluation,
			ClusterID:    "test",
			Namespace:    "default",
			Status:       v1alpha1.WorkflowRunning,
			Reason:       "",
			Image:        "test-image",
			RepoIds:      []string{"repo1"},
			RepoType:     "model",
			TaskDesc:     "test desc",
			ResourceId:   100,
			ResourceName: "test-resource",
			Datasets:     []string{"dataset1"},
		}

		updateWf := &v1alpha1.Workflow{
			ObjectMeta: v1.ObjectMeta{
				Name:      "test-task",
				Namespace: "default",
			},
			Status: v1alpha1.WorkflowStatus{
				Nodes: map[string]v1alpha1.NodeStatus{
					"test-task": {
						Phase:   v1alpha1.NodeFailed,
						Message: "workflow failed",
					},
				},
			},
		}

		argoStore.EXPECT().FindByTaskID(ctx, "test-task").Return(existingWf, nil)
		argoStore.EXPECT().UpdateWorkFlow(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			return wf.TaskId == "test-task" &&
				wf.Status == v1alpha1.WorkflowPhase(v1alpha1.NodeFailed)
		})).RunAndReturn(func(_ context.Context, wf database.ArgoWorkflow) (*database.ArgoWorkflow, error) {
			return &wf, nil
		})
		reporter.EXPECT().Report(mock.Anything)

		result, err := wfc.UpdateWorkflow(ctx, updateWf, cluster)
		require.Nil(t, err)
		require.Equal(t, v1alpha1.WorkflowPhase(v1alpha1.NodeFailed), result.Status)
	})

	t.Run("workflow with cluster node info", func(t *testing.T) {
		cluster := &cluster.Cluster{
			CID:           "test-cluster",
			ID:            "test",
			Client:        kubeClient,
			KnativeClient: knativefake.NewSimpleClientset(),
			ArgoClient:    argofake.NewSimpleClientset(),
		}

		existingWf := database.ArgoWorkflow{
			ID:           1,
			TaskId:       "test-task",
			Username:     "test-user",
			UserUUID:     "test-uuid",
			TaskName:     "test-task",
			TaskType:     types.TaskTypeEvaluation,
			ClusterID:    "test",
			Namespace:    "default",
			Status:       v1alpha1.WorkflowRunning,
			Reason:       "",
			Image:        "test-image",
			RepoIds:      []string{"repo1"},
			RepoType:     "model",
			TaskDesc:     "test desc",
			ResourceId:   100,
			ResourceName: "test-resource",
			Datasets:     []string{"dataset1"},
		}

		updateWf := &v1alpha1.Workflow{
			ObjectMeta: v1.ObjectMeta{
				Name:      "test-task",
				Namespace: "default",
			},
			Status: v1alpha1.WorkflowStatus{
				Nodes: map[string]v1alpha1.NodeStatus{
					"test-task": {
						Phase:   v1alpha1.NodeRunning,
						Message: "running",
					},
				},
			},
		}

		argoStore.EXPECT().FindByTaskID(ctx, "test-task").Return(existingWf, nil)
		argoStore.EXPECT().UpdateWorkFlow(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			return wf.TaskId == "test-task"
		})).RunAndReturn(func(_ context.Context, wf database.ArgoWorkflow) (*database.ArgoWorkflow, error) {
			return &wf, nil
		})

		result, err := wfc.UpdateWorkflow(ctx, updateWf, cluster)
		require.Nil(t, err)
		require.Equal(t, v1alpha1.WorkflowPhase(v1alpha1.NodeRunning), result.Status)
	})

	t.Run("update workflow with empty outputs", func(t *testing.T) {
		cluster := &cluster.Cluster{
			CID:           "test-cluster",
			ID:            "test",
			Client:        kubeClient,
			KnativeClient: knativefake.NewSimpleClientset(),
			ArgoClient:    argofake.NewSimpleClientset(),
		}

		existingWf := database.ArgoWorkflow{
			ID:           1,
			TaskId:       "test-task",
			Username:     "test-user",
			UserUUID:     "test-uuid",
			TaskName:     "test-task",
			TaskType:     types.TaskTypeEvaluation,
			ClusterID:    "test",
			Namespace:    "default",
			Status:       v1alpha1.WorkflowRunning,
			Reason:       "",
			Image:        "test-image",
			RepoIds:      []string{"repo1"},
			RepoType:     "model",
			TaskDesc:     "test desc",
			ResourceId:   100,
			ResourceName: "test-resource",
			Datasets:     []string{"dataset1"},
		}

		updateWf := &v1alpha1.Workflow{
			ObjectMeta: v1.ObjectMeta{
				Name:      "test-task",
				Namespace: "default",
			},
			Status: v1alpha1.WorkflowStatus{
				Nodes: map[string]v1alpha1.NodeStatus{
					"test-task": {
						Phase:   v1alpha1.NodeSucceeded,
						Message: "succeeded",
						Outputs: nil,
					},
				},
			},
		}

		argoStore.EXPECT().FindByTaskID(ctx, "test-task").Return(existingWf, nil)
		argoStore.EXPECT().UpdateWorkFlow(ctx, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
			return wf.TaskId == "test-task" &&
				wf.Status == v1alpha1.WorkflowPhase(v1alpha1.NodeSucceeded)
		})).RunAndReturn(func(_ context.Context, wf database.ArgoWorkflow) (*database.ArgoWorkflow, error) {
			return &wf, nil
		})
		reporter.EXPECT().Report(mock.Anything)

		result, err := wfc.UpdateWorkflow(ctx, updateWf, cluster)
		require.Nil(t, err)
		require.Equal(t, v1alpha1.WorkflowPhase(v1alpha1.NodeSucceeded), result.Status)
	})

	t.Run("database error when creating workflow from labels", func(t *testing.T) {
		cluster := &cluster.Cluster{
			CID:           "test-cluster",
			ID:            "test",
			Client:        kubeClient,
			KnativeClient: knativefake.NewSimpleClientset(),
			ArgoClient:    argofake.NewSimpleClientset(),
		}

		updateWf := &v1alpha1.Workflow{
			ObjectMeta: v1.ObjectMeta{
				Name:      "new-task",
				Namespace: "default",
				Annotations: map[string]string{
					"Username":     "test-user",
					"UserUUID":     "test-uuid",
					"TaskName":     "test-task",
					"TaskType":     string(types.TaskTypeEvaluation),
					"RepoIds":      "repo1",
					"TaskDesc":     "test desc",
					"Image":        "test-image",
					"Datasets":     "ds1",
					"ResourceId":   "100",
					"ResourceName": "test-resource",
					"ClusterID":    "test",
					"RepoType":     "model",
					"Namespace":    "default",
				},
			},
			Status: v1alpha1.WorkflowStatus{
				Nodes: map[string]v1alpha1.NodeStatus{
					"new-task": {
						Phase:   v1alpha1.NodeRunning,
						Message: "running",
					},
				},
			},
		}

		argoStore.EXPECT().FindByTaskID(ctx, "new-task").Return(database.ArgoWorkflow{}, sql.ErrNoRows)
		argoStore.EXPECT().CreateWorkFlow(ctx, mock.Anything).Return(nil, errors.New("create error"))

		_, err := wfc.UpdateWorkflow(ctx, updateWf, cluster)
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "failed to create workflow in db")
	})
}
