package component

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
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
	kubeClient := fake.NewClientset()
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

func TestGenerateWorkflow_FinetuneV2(t *testing.T) {
	req := types.ArgoWorkFlowReq{
		TaskId:          "ft-v2",
		TaskType:        types.TaskTypeFinetune,
		WorkflowVersion: 2,
		Entrypoint:      "finetune-v2",
		Nodes:           []types.Node{{Name: "gpu-node", HasXPU: true}},
		Templates: []types.ArgoFlowTemplate{
			{Name: "finetune", Image: "swift", HardWare: types.HardWare{Gpu: types.Processor{Num: "1", ResourceName: "nvidia.com/gpu"}}},
			{Name: "finetune-download", Image: "swift", HardWare: types.HardWare{Cpu: types.CPU{Num: "2"}}},
			{Name: "finetune-train", Image: "swift", HardWare: types.HardWare{Gpu: types.Processor{Num: "1", ResourceName: "nvidia.com/gpu"}}},
			{Name: "finetune-upload", Image: "swift", HardWare: types.HardWare{Cpu: types.CPU{Num: "2"}}},
			{Name: "finetune-cleanup", Image: "swift", HardWare: types.HardWare{Cpu: types.CPU{Num: "2"}}, Command: []string{"/etc/csghub/cleanup-job.sh"}},
		},
	}

	t.Run("uses staged DAG with shared PVC", func(t *testing.T) {
		wf, err := generateWorkflow(req, &config.Config{}, "user-pvc")
		require.NoError(t, err)
		require.Equal(t, "finetune-v2", wf.Spec.Entrypoint)
		require.Equal(t, "staged", wf.Annotations["FinetuneWorkflowMode"])
		require.Equal(t, "2", wf.Annotations["WorkflowVersion"])
		require.Len(t, wf.Spec.Volumes, 1)
		require.Len(t, wf.Spec.Templates, 5)

		dag := wf.Spec.Templates[4].DAG
		require.NotNil(t, dag)
		require.Equal(t, []string{"download"}, dag.Tasks[1].Dependencies)
		require.Equal(t, []string{"train"}, dag.Tasks[2].Dependencies)

		require.NotContains(t, wf.Spec.Templates[0].Container.Resources.Requests, corev1.ResourceName("nvidia.com/gpu"))
		require.Contains(t, wf.Spec.Templates[1].Container.Resources.Requests, corev1.ResourceName("nvidia.com/gpu"))
		require.NotContains(t, wf.Spec.Templates[2].Container.Resources.Requests, corev1.ResourceName("nvidia.com/gpu"))
		// cleanup template runs on CPU
		require.NotContains(t, wf.Spec.Templates[3].Container.Resources.Requests, corev1.ResourceName("nvidia.com/gpu"))

		// OnExit handler points to cleanup template
		require.Equal(t, "finetune-cleanup", wf.Spec.OnExit)
	})

	t.Run("falls back to original single container without PVC", func(t *testing.T) {
		wf, err := generateWorkflow(req, &config.Config{}, "")
		require.NoError(t, err)
		require.Equal(t, "finetune", wf.Spec.Entrypoint)
		require.Equal(t, "legacy-fallback", wf.Annotations["FinetuneWorkflowMode"])
		require.NotContains(t, wf.Annotations, "WorkflowVersion")
		require.Empty(t, wf.Spec.Volumes)
		require.Len(t, wf.Spec.Templates, 1)
		require.Equal(t, "finetune", wf.Spec.Templates[0].Name)
		// No OnExit handler for legacy fallback
		require.Empty(t, wf.Spec.OnExit)
	})
}

func TestFinetuneStageStatusesJSON(t *testing.T) {
	templates := []v1alpha1.Template{
		{
			Name: "finetune-download",
			Container: &corev1.Container{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				},
			},
		},
		{
			Name: "finetune-train",
			Container: &corev1.Container{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:                    resource.MustParse("4"),
						corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
					},
				},
			},
		},
	}

	t.Run("derives billability from requested resources", func(t *testing.T) {
		wf := &v1alpha1.Workflow{
			Spec: v1alpha1.WorkflowSpec{Templates: templates},
		}
		require.JSONEq(t,
			`{
				"finetune-download":{"phase":"Pending","billable":false},
				"finetune-train":{"phase":"Pending","billable":true}
			}`,
			finetuneStageStatusesJSON(wf),
		)
	})

	t.Run("reports the active pod phase without changing billability", func(t *testing.T) {
		wf := &v1alpha1.Workflow{
			Spec: v1alpha1.WorkflowSpec{Templates: templates},
			Status: v1alpha1.WorkflowStatus{
				Nodes: map[string]v1alpha1.NodeStatus{
					"train-pod": {
						TemplateName: "finetune-train",
						Phase:        v1alpha1.NodeRunning,
					},
				},
			},
		}
		require.JSONEq(t,
			`{
				"finetune-download":{"phase":"Pending","billable":false},
				"finetune-train":{"phase":"Running","billable":true}
			}`,
			finetuneStageStatusesJSON(wf),
		)
	})
}

func TestGetWorkflowFromLabels_FinetuneV2StageStatus(t *testing.T) {
	wfc := &workFlowComponentImpl{}
	wf := &v1alpha1.Workflow{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{
				"TaskType":        string(types.TaskTypeFinetune),
				"WorkflowVersion": "2",
			},
		},
		Spec: v1alpha1.WorkflowSpec{
			Templates: []v1alpha1.Template{
				{
					Name: "finetune-train",
					Container: &corev1.Container{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
							},
						},
					},
				},
			},
		},
		Status: v1alpha1.WorkflowStatus{
			Nodes: map[string]v1alpha1.NodeStatus{
				"train-pod": {
					TemplateName: "finetune-train",
					Phase:        v1alpha1.NodeSucceeded,
				},
			},
		},
	}

	got := wfc.getWorkflowFromLabels(context.Background(), wf)
	require.JSONEq(t,
		`{"finetune-train":{"phase":"Succeeded","billable":true}}`,
		got.DagTasks,
	)
}

func TestWorkFlowComponent_EnsureWorkflowPVC(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	wfc := &workFlowComponentImpl{}
	testCluster := &cluster.Cluster{
		Client:       kubeClient,
		StorageClass: "shared-rwx",
	}
	req := types.ArgoWorkFlowReq{
		Templates: []types.ArgoFlowTemplate{
			{
				Name: "finetune-train",
				HardWare: types.HardWare{
					EphemeralStorage: "80Gi",
				},
			},
		},
	}

	err := wfc.ensureWorkflowPVC(context.Background(), testCluster, "default", "user-pvc", req)
	require.NoError(t, err)

	pvc, err := kubeClient.CoreV1().PersistentVolumeClaims("default").Get(
		context.Background(), "user-pvc", v1.GetOptions{},
	)
	require.NoError(t, err)
	require.Equal(t, "shared-rwx", *pvc.Spec.StorageClassName)
	require.Equal(t, corev1.ReadWriteMany, pvc.Spec.AccessModes[0])
	require.Equal(t, "80Gi", pvc.Spec.Resources.Requests.Storage().String())

	require.NoError(t, wfc.ensureWorkflowPVC(context.Background(), testCluster, "default", "user-pvc", req))
}

func TestArgoComponent_DeleteWorkflow(t *testing.T) {
	argoStore := mockdb.NewMockArgoWorkFlowStore(t)
	pool := mockCluster.NewMockPool(t)
	kubeClient := fake.NewClientset()
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

// stagedFinetuneWorkflow builds a running v2 finetune workflow that keeps its
// data on the user shared PVC and cleans it up through spec.onExit.
func stagedFinetuneWorkflow(taskID, pvcName, workDir string) *v1alpha1.Workflow {
	return &v1alpha1.Workflow{
		ObjectMeta: v1.ObjectMeta{Name: taskID, Namespace: "test"},
		Spec: v1alpha1.WorkflowSpec{
			Entrypoint: "finetune-v2",
			OnExit:     finetuneCleanupTemplate,
			Templates: []v1alpha1.Template{
				{Name: "finetune-train", Container: &corev1.Container{Image: "swift"}},
				{Name: finetuneCleanupTemplate, Container: &corev1.Container{
					Image: "swift",
					Env:   []corev1.EnvVar{{Name: finetuneWorkDirEnv, Value: workDir}},
				}},
			},
			Volumes: []corev1.Volume{{
				Name: finetuneWorkspaceVolume,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
				},
			}},
		},
		Status: v1alpha1.WorkflowStatus{
			Phase: v1alpha1.WorkflowRunning,
			Nodes: v1alpha1.Nodes{
				taskID: {ID: taskID, Type: v1alpha1.NodeTypeDAG, Phase: v1alpha1.NodeRunning},
				"download": {
					ID:           "download",
					Type:         v1alpha1.NodeTypePod,
					TemplateName: "finetune-download",
					Phase:        v1alpha1.NodeSucceeded,
				},
			},
		},
	}
}

func newDeleteWorkflowTester(t *testing.T, awf *v1alpha1.Workflow, kubeObjects ...k8sruntime.Object) (workFlowComponentImpl, *argofake.Clientset, *types.ArgoWorkFlowDeleteReq) {
	t.Helper()
	pool := mockCluster.NewMockPool(t)
	argoClient := argofake.NewSimpleClientset(awf)
	pool.EXPECT().GetClusterByID(mock.Anything, "test").Return(&cluster.Cluster{
		CID:           "config",
		ID:            "test",
		Client:        fake.NewClientset(kubeObjects...),
		KnativeClient: knativefake.NewSimpleClientset(),
		ArgoClient:    argoClient,
	}, nil)
	wfc := workFlowComponentImpl{
		wf:          mockdb.NewMockArgoWorkFlowStore(t),
		clusterPool: pool,
		config:      &config.Config{},
		logReporter: mockReporter.NewMockLogCollector(t),
	}
	return wfc, argoClient, &types.ArgoWorkFlowDeleteReq{ID: 1, TaskID: awf.Name, ClusterID: "test", Namespace: "test"}
}

func userWorkspacePVC(name string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{ObjectMeta: v1.ObjectMeta{Name: name, Namespace: "test"}}
}

func getWorkflow(t *testing.T, client *argofake.Clientset, name string) (*v1alpha1.Workflow, error) {
	t.Helper()
	return client.ArgoprojV1alpha1().Workflows("test").Get(context.Background(), name, v1.GetOptions{})
}

func TestArgoComponent_DeleteWorkflow_StagedFinetuneCleanup(t *testing.T) {
	ctx := context.TODO()

	t.Run("stops workflow so onExit cleanup can release the task directory", func(t *testing.T) {
		awf := stagedFinetuneWorkflow("ft-v2", "user-pvc", "/workspace/finetune/ft-v2")
		wfc, argoClient, req := newDeleteWorkflowTester(t, awf, userWorkspacePVC("user-pvc"))

		require.NoError(t, wfc.DeleteWorkflow(ctx, req))

		stopped, err := getWorkflow(t, argoClient, "ft-v2")
		require.NoError(t, err)
		require.Equal(t, v1alpha1.ShutdownStrategyStop, stopped.Spec.Shutdown)
	})

	t.Run("keeps a workflow that is already stopping", func(t *testing.T) {
		awf := stagedFinetuneWorkflow("ft-v2", "user-pvc", "/workspace/finetune/ft-v2")
		awf.Spec.Shutdown = v1alpha1.ShutdownStrategyStop
		wfc, argoClient, req := newDeleteWorkflowTester(t, awf, userWorkspacePVC("user-pvc"))

		require.NoError(t, wfc.DeleteWorkflow(ctx, req))

		_, err := getWorkflow(t, argoClient, "ft-v2")
		require.NoError(t, err)
	})

	t.Run("deletes directly when the user has no shared PVC", func(t *testing.T) {
		awf := stagedFinetuneWorkflow("ft-v2", "user-pvc", "/workspace/finetune/ft-v2")
		wfc, argoClient, req := newDeleteWorkflowTester(t, awf)

		require.NoError(t, wfc.DeleteWorkflow(ctx, req))

		_, err := getWorkflow(t, argoClient, "ft-v2")
		require.True(t, k8serrors.IsNotFound(err))
	})

	t.Run("stops a workflow that reports no running pod", func(t *testing.T) {
		// Node status cannot rule out a task directory: argo creates the pod
		// before it persists the node describing it, it moves the node out of
		// Pending only once the controller observes the pod, it may compress or
		// offload the status out of the object, and it may create a pod between
		// this read and the delete. The download container meanwhile creates the
		// directory as its first statement, so every one of those shapes has to
		// go through the idempotent cleanup.
		for name, status := range map[string]v1alpha1.WorkflowStatus{
			"pending pod node": {
				Phase: v1alpha1.WorkflowPending,
				Nodes: v1alpha1.Nodes{
					"download": {ID: "download", Type: v1alpha1.NodeTypePod, Phase: v1alpha1.NodePending},
				},
			},
			"no node persisted yet": {Phase: v1alpha1.WorkflowPending},
			"node status offloaded": {
				Phase:                    v1alpha1.WorkflowRunning,
				OffloadNodeStatusVersion: "42",
			},
		} {
			t.Run(name, func(t *testing.T) {
				awf := stagedFinetuneWorkflow("ft-v2", "user-pvc", "/workspace/finetune/ft-v2")
				awf.Status = status
				wfc, argoClient, req := newDeleteWorkflowTester(t, awf, userWorkspacePVC("user-pvc"))

				require.NoError(t, wfc.DeleteWorkflow(ctx, req))

				stopped, err := getWorkflow(t, argoClient, "ft-v2")
				require.NoError(t, err)
				require.Equal(t, v1alpha1.ShutdownStrategyStop, stopped.Spec.Shutdown)
			})
		}
	})

	t.Run("keeps a stopping workflow whose node status is no longer readable", func(t *testing.T) {
		// A repeated cancel must not delete a workflow whose cleanup node is
		// still running, even when nothing about the run can be read back.
		awf := stagedFinetuneWorkflow("ft-v2", "user-pvc", "/workspace/finetune/ft-v2")
		awf.Spec.Shutdown = v1alpha1.ShutdownStrategyStop
		awf.Status.Nodes = nil
		wfc, argoClient, req := newDeleteWorkflowTester(t, awf, userWorkspacePVC("user-pvc"))

		require.NoError(t, wfc.DeleteWorkflow(ctx, req))

		_, err := getWorkflow(t, argoClient, "ft-v2")
		require.NoError(t, err)
	})

	t.Run("deletes a stopped workflow once its cleanup has finished", func(t *testing.T) {
		awf := stagedFinetuneWorkflow("ft-v2", "user-pvc", "/workspace/finetune/ft-v2")
		awf.Spec.Shutdown = v1alpha1.ShutdownStrategyStop
		awf.Status.Phase = v1alpha1.WorkflowFailed
		wfc, argoClient, req := newDeleteWorkflowTester(t, awf, userWorkspacePVC("user-pvc"))

		require.NoError(t, wfc.DeleteWorkflow(ctx, req))

		_, err := getWorkflow(t, argoClient, "ft-v2")
		require.True(t, k8serrors.IsNotFound(err))
	})

	t.Run("deletes directly when the workflow already ran its exit handler", func(t *testing.T) {
		awf := stagedFinetuneWorkflow("ft-v2", "user-pvc", "/workspace/finetune/ft-v2")
		awf.Status.Phase = v1alpha1.WorkflowSucceeded
		wfc, argoClient, req := newDeleteWorkflowTester(t, awf, userWorkspacePVC("user-pvc"))

		require.NoError(t, wfc.DeleteWorkflow(ctx, req))

		_, err := getWorkflow(t, argoClient, "ft-v2")
		require.True(t, k8serrors.IsNotFound(err))
	})

	t.Run("deletes directly when the cleanup directory is not owned by the task", func(t *testing.T) {
		awf := stagedFinetuneWorkflow("ft-v2", "user-pvc", "/workspace/finetune/other-task")
		wfc, argoClient, req := newDeleteWorkflowTester(t, awf, userWorkspacePVC("user-pvc"))

		require.NoError(t, wfc.DeleteWorkflow(ctx, req))

		_, err := getWorkflow(t, argoClient, "ft-v2")
		require.True(t, k8serrors.IsNotFound(err))
	})

	t.Run("deletes directly when the workflow has no cleanup exit handler", func(t *testing.T) {
		awf := stagedFinetuneWorkflow("ft-v2", "user-pvc", "/workspace/finetune/ft-v2")
		awf.Spec.OnExit = ""
		wfc, argoClient, req := newDeleteWorkflowTester(t, awf, userWorkspacePVC("user-pvc"))

		require.NoError(t, wfc.DeleteWorkflow(ctx, req))

		_, err := getWorkflow(t, argoClient, "ft-v2")
		require.True(t, k8serrors.IsNotFound(err))
	})
}

func TestFinetuneWorkDir(t *testing.T) {
	cases := []struct {
		name     string
		workDir  string
		expected string
	}{
		{name: "task directory", workDir: "/workspace/finetune/ft-v2", expected: "/workspace/finetune/ft-v2"},
		{name: "trailing slash", workDir: "/workspace/finetune/ft-v2/", expected: "/workspace/finetune/ft-v2"},
		{name: "mount root", workDir: "/workspace", expected: ""},
		{name: "other task", workDir: "/workspace/finetune/other", expected: ""},
		{name: "outside mount", workDir: "/data/finetune/ft-v2", expected: ""},
		{name: "empty", workDir: "", expected: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			awf := stagedFinetuneWorkflow("ft-v2", "user-pvc", c.workDir)
			require.Equal(t, c.expected, finetuneWorkDir(awf))
		})
	}
}

func TestArgoComponent_UpdateWorkflow(t *testing.T) {
	argoStore := mockdb.NewMockArgoWorkFlowStore(t)
	kubeClient := fake.NewClientset()
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

		existingWf := &database.ArgoWorkflow{
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

		existingWf := &database.ArgoWorkflow{
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

		existingWf := &database.ArgoWorkflow{
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

		existingWf := &database.ArgoWorkflow{
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

		existingWf := &database.ArgoWorkflow{
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

		argoStore.EXPECT().FindByTaskID(ctx, "new-task").Return(&database.ArgoWorkflow{}, sql.ErrNoRows)
		argoStore.EXPECT().CreateWorkFlow(ctx, mock.Anything).Return(nil, errors.New("create error"))

		_, err := wfc.UpdateWorkflow(ctx, updateWf, cluster)
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "failed to create workflow in db")
	})
}

func TestArgoComponent_UpdateWorkflow_KeepsCancelledStatus(t *testing.T) {
	argoStore := mockdb.NewMockArgoWorkFlowStore(t)
	wfc := workFlowComponentImpl{
		wf:          argoStore,
		config:      &config.Config{},
		logReporter: mockReporter.NewMockLogCollector(t),
	}
	ctx := context.TODO()
	cancelled := &database.ArgoWorkflow{
		ID:       1,
		TaskId:   "ft-v2",
		TaskType: types.TaskTypeFinetune,
		Status:   types.DFCancelled,
	}
	argoStore.EXPECT().FindByTaskID(ctx, "ft-v2").Return(cancelled, nil)

	// argo reports the stopped workflow as failed once the cleanup node exits
	update := &v1alpha1.Workflow{
		ObjectMeta: v1.ObjectMeta{Name: "ft-v2", Namespace: "test"},
		Status: v1alpha1.WorkflowStatus{
			Phase:   v1alpha1.WorkflowFailed,
			Message: "Stopped with strategy 'Stop'",
			Nodes: v1alpha1.Nodes{
				"ft-v2": {Phase: v1alpha1.NodeFailed},
			},
		},
	}

	result, err := wfc.UpdateWorkflow(ctx, update, &cluster.Cluster{
		ID:            "test",
		Client:        fake.NewClientset(),
		KnativeClient: knativefake.NewSimpleClientset(),
		ArgoClient:    argofake.NewSimpleClientset(),
	})
	require.NoError(t, err)
	require.Equal(t, v1alpha1.WorkflowPhase(types.DFCancelled), result.Status)
}
