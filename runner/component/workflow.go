package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	versioned "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-workflows/v3/pkg/client/informers/externalversions"
	internalinterfaces "github.com/argoproj/argo-workflows/v3/pkg/client/informers/externalversions/internalinterfaces"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type workFlowComponentImpl struct {
	config      *config.Config
	wf          database.ArgoWorkFlowStore
	clusterPool *cluster.ClusterPool
	eventPub    *event.EventPublisher
}

type WorkFlowComponent interface {
	// Create workflow
	CreateWorkflow(ctx context.Context, req types.ArgoWorkFlowReq) (*database.ArgoWorkflow, error)
	// Update workflow
	UpdateWorkflow(ctx context.Context, update *v1alpha1.Workflow) (*database.ArgoWorkflow, error)
	// find workflow by user name
	FindWorkFlows(ctx context.Context, username string, per, page int) ([]database.ArgoWorkflow, int, error)
	// generate workflow templates
	DeleteWorkflow(ctx context.Context, id int64, username string) error
	GetWorkflow(ctx context.Context, id int64, username string) (*database.ArgoWorkflow, error)
	DeleteWorkflowInargo(ctx context.Context, delete *v1alpha1.Workflow) error
	FindWorkFlowById(ctx context.Context, id int64) (database.ArgoWorkflow, error)
	RunWorkflowsInformer(clusterPool *cluster.ClusterPool, config *config.Config)
}

func NewWorkFlowComponent(config *config.Config, clusterPool *cluster.ClusterPool) WorkFlowComponent {
	wf := database.NewArgoWorkFlowStore()
	wc := &workFlowComponentImpl{
		config:      config,
		wf:          wf,
		clusterPool: clusterPool,
		eventPub:    &event.DefaultEventPublisher,
	}
	return wc
}

// Create workflow
func (wc *workFlowComponentImpl) CreateWorkflow(ctx context.Context, req types.ArgoWorkFlowReq) (*database.ArgoWorkflow, error) {
	// create workflow in db
	namespace := wc.config.Argo.Namespace
	if req.ShareMode {
		namespace = wc.config.Argo.QuotaNamespace
	}
	cluster, clusterId, err := GetCluster(ctx, wc.clusterPool, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster by id: %v", err)
	}
	argowf := &database.ArgoWorkflow{
		Username:     req.Username,
		UserUUID:     req.UserUUID,
		TaskName:     req.TaskName,
		TaskId:       req.TaskId,
		TaskType:     req.TaskType,
		RepoIds:      req.RepoIds,
		TaskDesc:     req.TaskDesc,
		Image:        req.Image,
		Datasets:     req.Datasets,
		ResourceId:   req.ResourceId,
		ResourceName: req.ResourceName,
		ClusterID:    clusterId,
		RepoType:     req.RepoType,
		Namespace:    namespace,
		Status:       v1alpha1.WorkflowPhase(v1alpha1.NodePending),
	}
	// create workflow in argo
	awf := generateWorkflow(req, wc.config)

	_, err = cluster.ArgoClient.ArgoprojV1alpha1().Workflows(namespace).Create(ctx, awf, v1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow in argo: %v", err)
	}
	wf, err := wc.wf.CreateWorkFlow(ctx, *argowf)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow in db: %v", err)
	}
	return wf, nil
}

func (wc *workFlowComponentImpl) DeleteWorkflow(ctx context.Context, id int64, username string) error {
	wf, err := wc.FindWorkFlowById(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get workflow by id: %v", err)
	}
	if wf.Username != username {
		return fmt.Errorf("no permission to delete workflow")
	}
	cluster, _, err := GetCluster(ctx, wc.clusterPool, wf.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster by id: %v", err)
	}
	err = cluster.ArgoClient.ArgoprojV1alpha1().Workflows(wf.Namespace).Delete(ctx, wf.TaskId, v1.DeleteOptions{})
	if err != nil {
		slog.Warn("Error deleting argo workflow", slog.Any("error", err))
	}
	return wc.wf.DeleteWorkFlow(ctx, wf.ID)
}

func (wc *workFlowComponentImpl) GetWorkflow(ctx context.Context, id int64, username string) (*database.ArgoWorkflow, error) {
	wf, err := wc.FindWorkFlowById(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow by id: %v", err)
	}
	if wf.Username != username {
		return nil, fmt.Errorf("no permission to get workflow")
	}
	return &wf, nil
}

// Update workflow
func (wc *workFlowComponentImpl) UpdateWorkflow(ctx context.Context, update *v1alpha1.Workflow) (*database.ArgoWorkflow, error) {
	oldwf, err := wc.wf.FindByTaskID(ctx, update.Name)
	if err != nil {
		return nil, err
	}

	oldwf.Reason = update.Status.Message
	if node, ok := update.Status.Nodes[oldwf.TaskId]; ok {
		oldwf.Status = v1alpha1.WorkflowPhase(node.Phase)
		if node.Phase == v1alpha1.NodeRunning {
			oldwf.StartTime = time.Now()
		}
		if _, exists := types.WorkFlowFinished[oldwf.Status]; exists {
			oldwf.EndTime = time.Now()
		}
		if node.Outputs != nil && node.Outputs.Parameters != nil {
			for _, output := range node.Outputs.Parameters {
				if output.Name == "result" && output.Value != nil {
					result := strings.Split(output.Value.String(), ",")
					oldwf.ResultURL = result[0]
					oldwf.DownloadURL = result[1]
					wc.StartAcctRequestFee(oldwf)
					break
				}
			}
		}
	}
	return wc.wf.UpdateWorkFlow(ctx, oldwf)
}

// DeleteWorkflowInargo
func (wc *workFlowComponentImpl) DeleteWorkflowInargo(ctx context.Context, delete *v1alpha1.Workflow) error {
	wf, err := wc.wf.FindByTaskID(ctx, delete.Name)
	if err != nil {
		return fmt.Errorf("failed to get workflow by id: %v", err)
	}
	// for deleted case,check if the workflow did not finish
	if wf.Status == v1alpha1.WorkflowPending || wf.Status == v1alpha1.WorkflowRunning {
		wf.Status = v1alpha1.WorkflowFailed
		wf.Reason = "deleted by admin"
		_, err = wc.wf.UpdateWorkFlow(ctx, wf)
		return err
	}
	return nil
}

func (wc *workFlowComponentImpl) FindWorkFlowById(ctx context.Context, id int64) (database.ArgoWorkflow, error) {
	return wc.wf.FindByID(ctx, id)
}

// find workflow by user name
func (wc *workFlowComponentImpl) FindWorkFlows(ctx context.Context, username string, per, page int) ([]database.ArgoWorkflow, int, error) {
	return wc.wf.FindByUsername(ctx, username, per, page)
}

// create workflow in argo
func generateWorkflow(req types.ArgoWorkFlowReq, config *config.Config) *v1alpha1.Workflow {
	templates := []v1alpha1.Template{}
	for _, v := range req.Templates {
		resReq, _ := GenerateResources(v.HardWare)
		environments := []corev1.EnvVar{}
		for key, value := range v.Env {
			environments = append(environments, corev1.EnvVar{Name: key, Value: value})
		}
		environments = append(environments, corev1.EnvVar{Name: "S3_ACCESS_ID", Value: config.S3.AccessKeyID})
		environments = append(environments, corev1.EnvVar{Name: "S3_ACCESS_SECRET", Value: config.S3.AccessKeySecret})
		environments = append(environments, corev1.EnvVar{Name: "S3_BUCKET", Value: config.Argo.S3PublicBucket})
		environments = append(environments, corev1.EnvVar{Name: "S3_ENDPOINT", Value: config.S3.Endpoint})
		environments = append(environments, corev1.EnvVar{Name: "S3_SSL_ENABLED", Value: strconv.FormatBool(config.S3.EnableSSL)})
		// fix no gpu request case
		if v.HardWare.Gpu.ResourceName == "" || v.HardWare.Gpu.Num == "" {
			environments = append(environments, corev1.EnvVar{Name: "NVIDIA_VISIBLE_DEVICES", Value: "none"})
		}

		resources := corev1.ResourceRequirements{
			Limits:   resReq,
			Requests: resReq,
		}

		containerImg := v.Image
		// add prefix if image is not full path
		if !strings.Contains(containerImg, "/") {
			if req.RepoType == string(types.ModelRepo) {
				// choose registry
				containerImg = path.Join(config.Model.DockerRegBase, v.Image)
			} else if req.RepoType == string(types.SpaceRepo) {
				// choose registry
				containerImg = path.Join(config.Space.DockerRegBase, v.Image)
			}
		}

		templates = append(templates, v1alpha1.Template{
			Name: v.Name,
			//NodeSelector: nodeSelector,
			Container: &corev1.Container{
				Image:     containerImg,
				Command:   v.Command,
				Env:       environments,
				Args:      v.Args,
				Resources: resources,
			},
			Outputs: v1alpha1.Outputs{
				Parameters: []v1alpha1.Parameter{
					{
						Name: "result",
						ValueFrom: &v1alpha1.ValueFrom{
							Path: "/tmp/output.txt",
						},
					},
				},
			},
		})
	}

	workflowObject := &v1alpha1.Workflow{
		ObjectMeta: v1.ObjectMeta{
			Name: req.TaskId,
			Labels: map[string]string{
				"workflow-scope": "csghub",
			},
		},
		Spec: v1alpha1.WorkflowSpec{
			Priority:           ptr.To(int32(3)),
			ServiceAccountName: config.Argo.ServiceAccountName,
			Templates:          templates,
			Entrypoint:         req.Entrypoint,
			TTLStrategy: &v1alpha1.TTLStrategy{
				// Set TTL here
				SecondsAfterCompletion: ptr.To(int32(config.Argo.JobTTL)),
			},
		},
	}

	return workflowObject
}

func (wc *workFlowComponentImpl) RunWorkflowsInformer(clusterPool *cluster.ClusterPool, c *config.Config) {
	labelSelector := "workflow-scope=csghub"
	clientset := clusterPool.Clusters[0].ArgoClient

	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// triggered in startup
			wf := obj.(*v1alpha1.Workflow)
			bg, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := wc.UpdateWorkflow(bg, wf)
			if err != nil {
				slog.Error("fail to update workflow", slog.Any("error", err), slog.Any("job id", wf.Name))
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldWF := oldObj.(*v1alpha1.Workflow)
			newWF := newObj.(*v1alpha1.Workflow)

			// compare status
			bg, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if newWF.Status.Nodes == nil || oldWF.Status.Nodes == nil {
				return
			}
			if oldWF.Status.Nodes[oldWF.Name].Phase != newWF.Status.Nodes[oldWF.Name].Phase {
				_, err := wc.UpdateWorkflow(bg, newWF)
				if err != nil {
					slog.Error("fail to update workflow", slog.Any("error", err), slog.Any("job id", newWF.Name))
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			//handle some special case
			wf := obj.(*v1alpha1.Workflow)
			bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := wc.DeleteWorkflowInargo(bg, wf)
			if err != nil {
				slog.Error("fail to update workflow", slog.Any("error", err), slog.Any("job id", wf.Name))
			}
		},
	}

	CreateInfomerFactory(clientset, labelSelector, wc.config.Argo.Namespace, eventHandler)

}

func CreateInfomerFactory(client versioned.Interface, namespace, labelSelector string, eventHandler cache.ResourceEventHandler) {
	f := externalversions.NewFilteredSharedInformerFactory(client, 2*time.Minute, namespace, internalinterfaces.TweakListOptionsFunc(func(list *v1.ListOptions) {
		list.LabelSelector = labelSelector
	}))

	informer := f.Argoproj().V1alpha1().Workflows().Informer()

	stopper := make(chan struct{})
	defer close(stopper)

	defer runtime.HandleCrash()
	_, _ = informer.AddEventHandler(eventHandler)

	informer.Run(stopper)
	if !cache.WaitForCacheSync(stopper, informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}
}

func (wc *workFlowComponentImpl) StartAcctRequestFee(wf database.ArgoWorkflow) {
	if !wc.config.IsMasterHost {
		return
	}
	if wf.ResourceId == 0 {
		return
	}
	duration := wf.EndTime.Sub(wf.StartTime)
	minutes := duration.Minutes()
	if minutes < 1 {
		return
	}
	slog.Info("start to acct request fee", slog.Any("mins", minutes))
	event := types.METERING_EVENT{
		Uuid:         uuid.New(),
		UserUUID:     wf.UserUUID,
		Value:        int64(minutes),
		ValueType:    types.TimeDurationMinType,
		Scene:        int(types.SceneEvaluation),
		OpUID:        "",
		ResourceID:   strconv.FormatInt(wf.ResourceId, 10),
		ResourceName: wf.ResourceName,
		CustomerID:   wf.TaskId,
		CreatedAt:    time.Now(),
	}
	str, err := json.Marshal(event)
	if err != nil {
		slog.Error("error marshal metering event", slog.Any("event", event), slog.Any("error", err))
		return
	}
	err = wc.eventPub.PublishMeteringEvent(str)
	if err != nil {
		slog.Error("failed to pub metering event", slog.Any("data", string(str)), slog.Any("error", err))
	} else {
		slog.Info("pub metering event success", slog.Any("data", string(str)))
	}
}

// get cluster
func GetCluster(ctx context.Context, clusterPool *cluster.ClusterPool, clusterID string) (*cluster.Cluster, string, error) {
	if clusterID == "" {
		clusterInfo, err := clusterPool.ClusterStore.ByClusterConfig(ctx, clusterPool.Clusters[0].CID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get cluster info: %v", err)
		}
		return &clusterPool.Clusters[0], clusterInfo.ClusterID, nil
	}
	cluster, err := clusterPool.GetClusterByID(ctx, clusterID)
	if err != nil {
		return nil, clusterID, fmt.Errorf("failed to get cluster by id: %v", err)
	}
	return cluster, clusterID, nil
}
