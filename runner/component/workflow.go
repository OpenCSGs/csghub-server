package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"opencsg.com/csghub-server/component/reporter"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	versioned "github.com/argoproj/argo-workflows/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-workflows/v3/pkg/client/informers/externalversions"
	internalinterfaces "github.com/argoproj/argo-workflows/v3/pkg/client/informers/externalversions/internalinterfaces"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/redis"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/runner/common"
	rcommon "opencsg.com/csghub-server/runner/common"
)

type workFlowComponentImpl struct {
	config      *config.Config
	wf          database.ArgoWorkFlowStore
	clusterPool *cluster.ClusterPool
	eventPub    *event.EventPublisher
	redisLocker *redis.DistributedLocker
	logReporter reporter.LogCollector
}

type WorkFlowComponent interface {
	// Create workflow
	CreateWorkflow(ctx context.Context, req types.ArgoWorkFlowReq) (*database.ArgoWorkflow, error)
	// Update workflow
	UpdateWorkflow(ctx context.Context, update *v1alpha1.Workflow, cluster *cluster.Cluster) (*database.ArgoWorkflow, error)
	// find workflow by user name
	FindWorkFlows(ctx context.Context, username string, per, page int) ([]database.ArgoWorkflow, int, error)
	// generate workflow templates
	DeleteWorkflow(ctx context.Context, req *types.ArgoWorkFlowDeleteReq) error
	GetWorkflow(ctx context.Context, id int64, username string) (*database.ArgoWorkflow, error)
	DeleteWorkflowInargo(ctx context.Context, delete *v1alpha1.Workflow) error
	FindWorkFlowById(ctx context.Context, id int64) (database.ArgoWorkflow, error)
	RunInformer(clusterPool *cluster.ClusterPool, config *config.Config)
}

func NewWorkFlowComponent(config *config.Config, clusterPool *cluster.ClusterPool, logReporter reporter.LogCollector) WorkFlowComponent {
	wf := database.NewArgoWorkFlowStore()
	wc := &workFlowComponentImpl{
		config:      config,
		wf:          wf,
		clusterPool: clusterPool,
		eventPub:    &event.DefaultEventPublisher,
		redisLocker: redis.InitDistributedLocker(config),
		logReporter: logReporter,
	}
	return wc
}

// Create workflow
func (wc *workFlowComponentImpl) CreateWorkflow(ctx context.Context, req types.ArgoWorkFlowReq) (*database.ArgoWorkflow, error) {
	// create workflow in db
	namespace := wc.config.Cluster.SpaceNamespace
	if req.ShareMode {
		namespace = wc.config.Cluster.ResourceQuotaNamespace
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
	wc.setLabels(argowf, awf)

	_, err = cluster.ArgoClient.ArgoprojV1alpha1().Workflows(namespace).Create(ctx, awf, v1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow in argo: %v", err)
	}
	wf, err := wc.wf.CreateWorkFlow(ctx, *argowf)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow in db: %v", err)
	}

	wc.addKServiceWithEvent(ctx, types.RunnerWorkflowCreate, argowf)
	wc.logReporter.Report(types.LogEntry{
		Message:  "succeeded create ksvc",
		Stage:    types.StageDeploy,
		Step:     types.StepDeployRunning,
		DeployID: req.TaskId,
		Labels: map[string]string{
			types.LogLabelTypeKey:       types.LogLabelDeploy,
			types.StreamKeyDeployTypeID: req.TaskId,
			types.StreamKeyDeployType:   string(req.TaskType),
		},
	})
	return wf, nil
}

func (wc *workFlowComponentImpl) DeleteWorkflow(ctx context.Context, req *types.ArgoWorkFlowDeleteReq) error {
	cluster, _, err := GetCluster(ctx, wc.clusterPool, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster by id: %v", err)
	}
	err = cluster.ArgoClient.ArgoprojV1alpha1().Workflows(req.Namespace).Delete(ctx, req.TaskID, v1.DeleteOptions{})
	if err != nil {
		slog.Warn("Error deleting argo workflow", slog.Any("error", err))
	}
	return nil
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
func (wc *workFlowComponentImpl) UpdateWorkflow(ctx context.Context, update *v1alpha1.Workflow, cluster *cluster.Cluster) (*database.ArgoWorkflow, error) {
	oldwf, err := wc.wf.FindByTaskID(ctx, update.Name)
	if errors.Is(err, sql.ErrNoRows) {
		oldwf = *wc.getWorkflowFromLabels(ctx, update)
		wf, err := wc.wf.CreateWorkFlow(ctx, oldwf)
		if err != nil {
			slog.Error("failed to create workflow in db", slog.Any("error", err))
			return nil, fmt.Errorf("failed to create workflow in db: %v", err)
		}
		oldwf = *wf
	}
	if err != nil {
		return nil, err
	}

	lastStatus := oldwf.Status
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
					break
				}
			}
		}
		//if oldwf.Status is error, get the log from the pod and save to reason field
		if oldwf.Status == v1alpha1.WorkflowFailed || oldwf.Status == v1alpha1.WorkflowError {
			//podName := fmt.Sprintf("%s-%s", oldwf.TaskId, oldwf.ClusterID)
			logs, err := common.GetPodLog(ctx, cluster.Client, update.Name, update.Namespace, "main")
			if err != nil {
				slog.Error("failed to get pod log", slog.Any("error", err), slog.Any("pod name", update.Name))
			} else {
				if len(logs) > 0 {
					oldwf.Reason = string(logs)
				}
			}

		}
	}

	wc.addKServiceWithEvent(ctx, types.RunnerWorkflowChange, &oldwf)
	if lastStatus != oldwf.Status {
		wc.reportWorFlowLog(types.WorkflowUpdated.String(), &oldwf)
	}
	return wc.wf.UpdateWorkFlow(ctx, oldwf)
}

// DeleteWorkflowInargo
func (wc *workFlowComponentImpl) DeleteWorkflowInargo(ctx context.Context, delete *v1alpha1.Workflow) error {
	wf, err := wc.wf.FindByTaskID(ctx, delete.Name)
	if err != nil {
		return fmt.Errorf("failed to get workflow by id: %v", err)
	}

	wc.reportWorFlowLog(types.WorkflowDeleted.String(), &wf)
	// for deleted case,check if the workflow did not finish
	if wf.Status == v1alpha1.WorkflowPending || wf.Status == v1alpha1.WorkflowRunning {
		wf.Status = v1alpha1.WorkflowFailed
		wf.Reason = "deleted by admin"
		_, err = wc.wf.UpdateWorkFlow(ctx, wf)
		if err != nil {
			return err
		}

		wc.addKServiceWithEvent(ctx, types.RunnerWorkflowChange, &wf)
		return nil
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
		if v.HardWare.Npu.ResourceName == "" || v.HardWare.Npu.Num == "" {
			environments = append(environments, corev1.EnvVar{Name: "ASCEND_VISIBLE_DEVICES", Value: "none"})
		}

		if v.HardWare.Dcu.ResourceName == "" || v.HardWare.Dcu.Num == "" {
			environments = append(environments, corev1.EnvVar{Name: "ENFLAME_VISIBLE_DEVICES", Value: "none"})
		}

		if v.HardWare.Gcu.ResourceName == "" || v.HardWare.Gcu.Num == "" {
			environments = append(environments, corev1.EnvVar{Name: "ROCR_VISIBLE_DEVICES", Value: "none"})
		}

		resources := corev1.ResourceRequirements{
			Limits:   resReq,
			Requests: resReq,
		}

		containerImg := v.Image
		// add prefix if image is not full path

		if req.RepoType == string(types.ModelRepo) {
			// choose registry
			if strings.Count(containerImg, "/") == 1 {
				containerImg = path.Join(config.Model.DockerRegBase, v.Image)
			}
		} else if req.RepoType == string(types.SpaceRepo) {
			// choose registry
			containerImg = path.Join(config.Space.DockerRegBase, v.Image)
		}

		templates = append(templates, v1alpha1.Template{
			Name: v.Name,
			//NodeSelector: nodeSelector,
			Container: &corev1.Container{
				Image:           containerImg,
				Command:         v.Command,
				Env:             environments,
				Args:            v.Args,
				Resources:       resources,
				ImagePullPolicy: corev1.PullAlways,
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
			PodMetadata: &v1alpha1.Metadata{
				Labels: map[string]string{
					types.LogLabelTypeKey:       types.LogLabelDeploy,
					types.StreamKeyDeployID:     req.TaskId, // should be replace with deployID
					types.StreamKeyDeployTypeID: req.TaskId,
				},
			},
		},
	}

	return workflowObject
}

func (wc *workFlowComponentImpl) RunInformer(clusterPool *cluster.ClusterPool, c *config.Config) {
	var wg sync.WaitGroup
	stopCh := make(chan struct{})
	defer close(stopCh)
	defer runtime.HandleCrash()
	for _, cls := range clusterPool.Clusters {
		_, err := cls.Client.Discovery().ServerVersion()
		if err != nil {
			slog.Error("cluster is unavailable ", slog.Any("cluster config", cls.CID), slog.Any("error", err))
			continue
		}
		if wc.config.Cluster.ResourceQuotaNamespace != wc.config.Cluster.SpaceNamespace {
			wg.Add(2)
			go func(cluster *cluster.Cluster) {
				defer wg.Done()
				go wc.RunArgoInformer(stopCh, wc.config.Cluster.SpaceNamespace, cls)
			}(cls)
			go func(cluster *cluster.Cluster) {
				defer wg.Done()
				go wc.RunArgoInformer(stopCh, wc.config.Cluster.ResourceQuotaNamespace, cls)
			}(cls)

		} else {
			wg.Add(1)
			go func(cluster *cluster.Cluster) {
				defer wg.Done()
				go wc.RunArgoInformer(stopCh, wc.config.Cluster.SpaceNamespace, cls)
			}(cls)
		}
	}
	wg.Wait()
}

func (wc *workFlowComponentImpl) RunArgoInformer(stopCh <-chan struct{}, namespace string, cluster *cluster.Cluster) {
	labelSelector := "workflow-scope=csghub"
	client := cluster.ArgoClient
	f := externalversions.NewFilteredSharedInformerFactory(client, 2*time.Minute, namespace, internalinterfaces.TweakListOptionsFunc(func(list *v1.ListOptions) {
		list.LabelSelector = labelSelector
	}))

	informer := f.Argoproj().V1alpha1().Workflows().Informer()

	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// triggered in startup
			wf := obj.(*v1alpha1.Workflow)
			bg, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := wc.UpdateWorkflow(bg, wf, cluster)
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
				_, err := wc.UpdateWorkflow(bg, newWF, cluster)
				if err != nil {
					slog.Error("fail to update workflow", slog.Any("error", err), slog.Any("job id", newWF.Name))
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			//handle some special case
			switch wf := obj.(type) {
			case *v1alpha1.Workflow:
				bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err := wc.DeleteWorkflowInargo(bg, wf)
				if err != nil {
					slog.Error("fail to update workflow", slog.Any("error", err), slog.Any("job id", wf.Name))
				}
			default:
				slog.Error("unknown type", slog.Any("type", wf))
				return
			}
		},
	}

	stopper := make(chan struct{})
	defer close(stopper)

	defer runtime.HandleCrash()
	_, err := informer.AddEventHandler(eventHandler)
	if err != nil {
		runtime.HandleError(fmt.Errorf("failed to add event handler for argo workflow informer"))
	}
	informer.Run(stopper)
	if !cache.WaitForCacheSync(stopper, informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}
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

// get cluster
func GetCluster(ctx context.Context, clusterPool *cluster.ClusterPool, clusterID string) (*cluster.Cluster, string, error) {
	if clusterID == "" {
		clusterInfo, err := clusterPool.ClusterStore.ByClusterConfig(ctx, clusterPool.Clusters[0].CID)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get cluster info: %v", err)
		}
		return clusterPool.Clusters[0], clusterInfo.ClusterID, nil
	}
	cluster, err := clusterPool.GetClusterByID(ctx, clusterID)
	if err != nil {
		return nil, clusterID, fmt.Errorf("failed to get cluster by id: %v", err)
	}
	return cluster, clusterID, nil
}

func (wc *workFlowComponentImpl) setLabels(wf *database.ArgoWorkflow, awf *v1alpha1.Workflow) {
	if awf.ObjectMeta.Annotations == nil {
		awf.ObjectMeta.Annotations = make(map[string]string)
	}
	awf.ObjectMeta.Annotations["Username"] = wf.Username
	awf.ObjectMeta.Annotations["UserUUID"] = wf.UserUUID
	awf.ObjectMeta.Annotations["TaskName"] = wf.TaskName
	awf.ObjectMeta.Annotations["TaskId"] = wf.TaskId
	awf.ObjectMeta.Annotations["TaskType"] = string(wf.TaskType)
	awf.ObjectMeta.Annotations["RepoIds"] = strings.Join(wf.RepoIds, ",")
	awf.ObjectMeta.Annotations["TaskDesc"] = wf.TaskDesc
	awf.ObjectMeta.Annotations["Image"] = wf.Image
	awf.ObjectMeta.Annotations["Datasets"] = strings.Join(wf.Datasets, ",")
	awf.ObjectMeta.Annotations["ResourceId"] = strconv.FormatInt(wf.ResourceId, 10)
	awf.ObjectMeta.Annotations["ResourceName"] = wf.ResourceName
	awf.ObjectMeta.Annotations["ClusterID"] = wf.ClusterID
	awf.ObjectMeta.Annotations["RepoType"] = wf.RepoType
	awf.ObjectMeta.Annotations["Namespace"] = wf.Namespace
}

func (wc *workFlowComponentImpl) getWorkflowFromLabels(ctx context.Context, awf *v1alpha1.Workflow) *database.ArgoWorkflow {
	wf := &database.ArgoWorkflow{}
	annotations := awf.ObjectMeta.Annotations
	// Basic string fields
	wf.Username = annotations["Username"]
	wf.UserUUID = annotations["UserUUID"]
	wf.TaskName = annotations["TaskName"]
	wf.TaskId = annotations["TaskId"]
	wf.TaskType = types.TaskType(annotations["TaskType"])
	wf.TaskDesc = annotations["TaskDesc"]
	wf.Image = annotations["Image"]
	wf.ResourceName = annotations["ResourceName"]
	wf.ClusterID = annotations["ClusterID"]
	wf.RepoType = annotations["RepoType"]
	wf.Namespace = annotations["Namespace"]

	// String slice fields
	if repoIdsStr, ok := annotations["RepoIds"]; ok && repoIdsStr != "" {
		wf.RepoIds = strings.Split(repoIdsStr, ",")
	}
	if datasetsStr, ok := annotations["Datasets"]; ok && datasetsStr != "" {
		wf.Datasets = strings.Split(datasetsStr, ",")
	}

	// Numeric fields
	if resourceIdStr, ok := annotations["ResourceId"]; ok && resourceIdStr != "" {
		resourceId, err := strconv.ParseInt(resourceIdStr, 10, 64)
		if err == nil {
			wf.ResourceId = resourceId
		}
	}

	wf.Status = awf.Status.Phase

	return wf
}

func (wc *workFlowComponentImpl) addKServiceWithEvent(ctx context.Context, eventType types.WebHookEventType, wf *database.ArgoWorkflow) {
	event := &types.WebHookSendEvent{
		WebHookHeader: types.WebHookHeader{
			EventType: eventType,
			EventTime: time.Now().Unix(),
			ClusterID: wf.ClusterID,
			DataType:  types.WebHookDataTypeObject,
		},
		Data: wf,
	}

	go func() {
		err := rcommon.Push(wc.config.Runner.WebHookEndpoint, wc.config.APIToken, event)
		if err != nil {
			slog.Error("failed to push workflow service status event", slog.Any("error", err))
		}
	}()
}

func (s *workFlowComponentImpl) reportWorFlowLog(msg string, wf *database.ArgoWorkflow) {
	logEntry := types.LogEntry{
		Message: fmt.Sprintf("%s, argo workflow statue: %s", msg, wf.Status),
		Stage:   types.StageDeploy,
		Step:    types.StepDeployRunning,
		// should be deployID;
		DeployID: wf.TaskId,
		Labels: map[string]string{
			types.LogLabelTypeKey:       types.LogLabelDeploy,
			types.LogLabelKeyClusterID:  wf.ClusterID,
			types.StreamKeyDeployType:   string(wf.TaskType),
			types.StreamKeyDeployTypeID: strconv.FormatInt(wf.ID, 10),
		},
	}
	s.logReporter.Report(logEntry)
}
