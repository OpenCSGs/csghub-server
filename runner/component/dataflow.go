package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/argoproj/argo-workflows/v3/pkg/client/informers/externalversions"
	internalinterfaces "github.com/argoproj/argo-workflows/v3/pkg/client/informers/externalversions/internalinterfaces"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/runner/common"
	sched "opencsg.com/csghub-server/runner/component/kube_scheduler"
	rtypes "opencsg.com/csghub-server/runner/types"
	"opencsg.com/csghub-server/runner/utils"
)

type DataflowComponent interface {
	CreateWorkflow(ctx context.Context, req *types.DataflowArgoJobReq) (*types.DataflowArgoJobResp, error)
	GetStatus(ctx context.Context, req *types.DataflowArgoReq) (*types.DataflowArgoJobResp, error)
	DeleteWorkflow(ctx context.Context, req *types.DataflowArgoReq) error
	RunInformer()
}

type dataflowComponentImpl struct {
	config      *config.Config
	clusterPool cluster.Pool
	namespace   string
	wfStore     database.ArgoWorkFlowStore
}

func NewDataflowComponent(config *config.Config, clusterPool cluster.Pool) DataflowComponent {
	df := &dataflowComponentImpl{
		config:      config,
		clusterPool: clusterPool,
		namespace:   config.Cluster.SpaceNamespace,
		wfStore:     database.NewArgoWorkFlowStore(),
	}
	go df.RunInformer()
	return df
}

func (d *dataflowComponentImpl) CreateWorkflow(ctx context.Context, req *types.DataflowArgoJobReq) (*types.DataflowArgoJobResp, error) {
	cluster, err := d.clusterPool.GetClusterByID(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %s for dataflow job %s error: %w", req.ClusterID, req.JobID, err)
	}

	dwf, err := d.buildWorkflow(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build dataflow workflow job %s error: %w", req.JobID, err)
	}

	if err := d.createPVC(ctx, cluster, req); err != nil {
		return nil, fmt.Errorf("failed to create pvc for dataflow workflow job %s error: %w", req.JobID, err)
	}

	dfWorkflow, err := cluster.ArgoClient.ArgoprojV1alpha1().Workflows(d.namespace).Create(ctx, dwf, v1.CreateOptions{})
	if err != nil {
		delErr := d.deletePVC(ctx, cluster, &types.DataflowArgoReq{ClusterID: req.ClusterID, ArgoTaskID: req.ArgoTaskID})
		if delErr != nil {
			slog.ErrorContext(ctx, "delete pvc due to create dataflow workflow job %s failed error: %w", req.ArgoTaskID, delErr)
		}
		return nil, fmt.Errorf("failed to create dataflow workflow job %s error: %w", req.JobID, err)
	}
	slog.InfoContext(ctx, "create dataflow workflow success",
		slog.String("namespace", d.namespace), slog.String("name", dfWorkflow.Name))

	return &types.DataflowArgoJobResp{
		ID:         req.ID,
		ArgoTaskID: dfWorkflow.Name,
		JobID:      req.JobID,
		JobName:    req.JobName,
		Status:     string(v1alpha1.WorkflowPending),
		Message:    dfWorkflow.Status.Message,
		CreatedAt:  dfWorkflow.CreationTimestamp.Unix(),
	}, nil
}

func genPVCName(taskID string) string {
	return types.DFPVCNamePrefix + taskID
}

func (d *dataflowComponentImpl) createPVC(ctx context.Context, cluster *cluster.Cluster, req *types.DataflowArgoJobReq) error {
	pvcName := genPVCName(req.ArgoTaskID)
	_, err := cluster.Client.CoreV1().PersistentVolumeClaims(d.namespace).Get(ctx, pvcName, v1.GetOptions{})
	if err == nil {
		slog.WarnContext(ctx, "pvc already exists", slog.Any("pvcName", pvcName),
			slog.Any("argoTaskID", req.ArgoTaskID), slog.Any("jobid", req.JobID))
		return nil
	}

	storageSize, err := resource.ParseQuantity(req.StorageSize)
	if err != nil {
		return fmt.Errorf("failed to parse storage size %s for dataflow job %s, taskid %s, error: %w", req.StorageSize, req.JobID, req.ArgoTaskID, err)
	}

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Namespace: d.namespace,
			Name:      pvcName,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageSize,
				},
			},
			StorageClassName: &cluster.StorageClass,
		},
	}

	_, err = cluster.Client.CoreV1().PersistentVolumeClaims(d.namespace).Create(ctx, &pvc, v1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pvc %s for dataflow job %s failed: %w", pvcName, req.ArgoTaskID, err)
	}

	return nil
}

func (d *dataflowComponentImpl) deletePVC(ctx context.Context, cluster *cluster.Cluster, req *types.DataflowArgoReq) error {
	pvcName := genPVCName(req.ArgoTaskID)
	err := cluster.Client.CoreV1().PersistentVolumeClaims(d.namespace).Delete(ctx, pvcName, v1.DeleteOptions{})
	return err
}

func (d *dataflowComponentImpl) buildWorkflow(req *types.DataflowArgoJobReq) (*v1alpha1.Workflow, error) {
	applier := sched.NewApplier(req.Scheduler)
	deployExt := types.DeployExtend{
		NodeAffinity: req.NodeAffinity,
		Tolerations:  req.Tolerations,
	}
	genRes := common.GenerateResources(rtypes.ResourceGeneratorParams{
		Hardware:  req.Template.HardWare,
		Nodes:     req.Nodes,
		DeployExt: deployExt,
		Config:    d.config,
	})
	resReq, nodeAffinity := genRes.ResourceRequirements, genRes.NodeAffinity
	resources := corev1.ResourceRequirements{
		Limits:   resReq,
		Requests: resReq,
	}
	annotations := map[string]string{
		types.DFUniqueIDKey:     fmt.Sprintf("%d", req.ID),
		types.DFJobIDKey:        req.JobID,
		types.DFJobNameKey:      req.JobName,
		types.DFArgoTaskIDKey:   req.ArgoTaskID,
		types.DFOpUserUUIDKey:   req.OpUserUUID,
		types.DFOpUserNameKey:   req.Username,
		types.DFNSUUIDKey:       req.NSUUID,
		types.DFClusterIDKey:    req.ClusterID,
		types.DFResourceIDKey:   fmt.Sprintf("%d", req.ResourceId),
		types.DFResourceNameKey: req.ResourceName,
		types.DFJobDescKey:      req.JobDesc,
		types.DFImageKey:        req.Template.Image,
		types.DFStorageSizeKey:  req.StorageSize,
	}

	templates := []v1alpha1.Template{}
	volumeName := "workflow-data"

	volumes := []corev1.Volume{
		{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: genPVCName(req.ArgoTaskID),
					ReadOnly:  false,
				},
			},
		},
	}

	runtimeTemp := d.buildRuntimeTemplate(volumeName, annotations, resources, req)

	// merge node affinity
	utils.FillAffinity(&runtimeTemp.Affinity, nodeAffinity)
	// fill tolerations
	if len(genRes.Tolerations) > 0 {
		runtimeTemp.Tolerations = genRes.Tolerations
	}
	if err := applier.ApplyToArgo(runtimeTemp); err != nil {
		return nil, fmt.Errorf("failed to apply scheduler to dataflow runtime template: %v", err)
	}

	dagTemp := d.buildDAGTemplate(req)

	templates = append(templates, *runtimeTemp)
	templates = append(templates, *dagTemp)

	dataflowObject := &v1alpha1.Workflow{
		ObjectMeta: v1.ObjectMeta{
			Namespace:   d.namespace,
			Name:        req.ArgoTaskID,
			Annotations: annotations,
			Labels: map[string]string{
				types.DFUniqueIDKey:   fmt.Sprintf("%d", req.ID),
				types.DFLabelTagKey:   types.DFLabelTagValue,
				types.DFJobIDKey:      req.JobID,
				types.DFArgoTaskIDKey: req.ArgoTaskID,
			},
		},
		Spec: v1alpha1.WorkflowSpec{
			ServiceAccountName: d.config.Argo.ServiceAccountName,
			Entrypoint:         req.Entrypoint,
			Volumes:            volumes,
			Templates:          templates,
			TTLStrategy: &v1alpha1.TTLStrategy{
				// Set TTL here
				SecondsAfterCompletion: ptr.To(int32(d.config.Argo.JobTTL)),
			},
		},
	}

	return dataflowObject, nil
}

func (d *dataflowComponentImpl) buildRuntimeTemplate(
	volumeName string,
	annotations map[string]string,
	resources corev1.ResourceRequirements,
	req *types.DataflowArgoJobReq) *v1alpha1.Template {
	containerImg := path.Join(d.config.Model.DockerRegBase, req.Template.Image)

	params := []v1alpha1.Parameter{}
	for _, param := range req.Template.Parameters {
		params = append(params, v1alpha1.Parameter{
			Name: param,
		})
	}
	params = append(params,
		v1alpha1.Parameter{
			Name: types.DFParamDagTaskIDKey,
		}, v1alpha1.Parameter{
			Name: types.DFParamDagTaskNameKey,
		},
	)
	dataflowDataPath := "/data/dataflow_data"
	// Build environment variables from template.Env, req.Env and AccessToken
	environments := []corev1.EnvVar{}
	// Inject env variables from template
	if req.Template.Env != nil {
		value, ok := req.Template.Env[types.DataflowDataPathKey]
		if ok && len(value) > 0 {
			dataflowDataPath = value
		}
		for key, value := range req.Template.Env {
			environments = append(environments, corev1.EnvVar{Name: key, Value: value})
		}
	}
	// Inject user's access token into pod environment
	if len(req.AccessToken) > 0 {
		environments = append(environments, corev1.EnvVar{Name: types.AccessTokenEnvKey, Value: req.AccessToken})
	}

	runtimeTemp := &v1alpha1.Template{
		Name: req.Template.Name, // "echo"
		Inputs: v1alpha1.Inputs{
			Parameters: params, // []v1alpha1.Parameter{ { Name: "cmd" }, { Name: "task_id" } },
		},
		Metadata: v1alpha1.Metadata{
			Annotations: annotations,
			Labels: map[string]string{
				types.DFArgoTaskIDKey:       req.ArgoTaskID,
				types.DFUniqueIDKey:         fmt.Sprintf("%d", req.ID),
				types.DFLabelTagKey:         types.DFLabelTagValue,
				types.DFJobIDKey:            req.JobID,
				types.DFLabelDagTaskIDKey:   fmt.Sprintf("{{inputs.parameters.%s}}", types.DFParamDagTaskIDKey),
				types.DFLabelDagTaskNameKey: fmt.Sprintf("{{inputs.parameters.%s}}", types.DFParamDagTaskNameKey),
				types.StreamKeyDeployID:     req.ArgoTaskID,
			},
		},
		Container: &corev1.Container{
			// example: "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/alpine:latest",
			Image:   containerImg,
			Command: req.Template.Command, // []string{"sh", "-c"},
			Args:    req.Template.Args,    // []string{"{{inputs.parameters.cmd}}"},
			Env:     environments,
			VolumeMounts: []corev1.VolumeMount{
				{Name: volumeName, MountPath: dataflowDataPath},
			},
			Resources:       resources,
			ImagePullPolicy: corev1.PullAlways,
		},
	}

	return runtimeTemp
}

func (d *dataflowComponentImpl) buildDAGTemplate(req *types.DataflowArgoJobReq) *v1alpha1.Template {
	tasks := []v1alpha1.DAGTask{}
	for _, task := range req.DagTasks {
		taskParams := []v1alpha1.Parameter{}
		taskParams = append(taskParams,
			v1alpha1.Parameter{
				Name:  types.DFParamDagTaskIDKey,
				Value: v1alpha1.AnyStringPtr(task.ID),
			},
			v1alpha1.Parameter{
				Name:  types.DFParamDagTaskNameKey,
				Value: v1alpha1.AnyStringPtr(task.Name),
			},
		)
		for _, param := range task.Parameters {
			taskParams = append(taskParams, v1alpha1.Parameter{
				Name:  param.Name,
				Value: v1alpha1.AnyStringPtr(param.Value),
			})
		}
		tasks = append(tasks, v1alpha1.DAGTask{
			Name:         task.Name,
			Template:     task.Template,
			Dependencies: task.Deps,
			Arguments: v1alpha1.Arguments{
				Parameters: taskParams,
			},
		})
	}

	dagTemp := &v1alpha1.Template{
		Name: req.Entrypoint,
		DAG: &v1alpha1.DAGTemplate{
			Tasks: tasks,
		},
	}

	return dagTemp
}

func (d *dataflowComponentImpl) GetStatus(ctx context.Context, req *types.DataflowArgoReq) (*types.DataflowArgoJobResp, error) {
	cluster, err := d.clusterPool.GetClusterByID(ctx, req.ClusterID)
	if err != nil {
		return nil, err
	}

	workflow, err := cluster.ArgoClient.ArgoprojV1alpha1().Workflows(d.namespace).Get(ctx, req.ArgoTaskID, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("dataflow %s workflow not found error: %w", req.ArgoTaskID, err)
	}

	return &types.DataflowArgoJobResp{
		ArgoTaskID: req.ArgoTaskID,
		JobID:      workflow.Annotations[types.DFJobIDKey],
		JobName:    workflow.Annotations[types.DFJobNameKey],
		Status:     string(workflow.Status.Phase),
		Message:    workflow.Status.Message,
		CreatedAt:  workflow.CreationTimestamp.Unix(),
	}, nil
}

func (d *dataflowComponentImpl) DeleteWorkflow(ctx context.Context, req *types.DataflowArgoReq) error {
	cluster, err := d.clusterPool.GetClusterByID(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster %s error: %w", req.ClusterID, err)
	}

	err = cluster.ArgoClient.ArgoprojV1alpha1().Workflows(d.namespace).Delete(ctx, req.ArgoTaskID, v1.DeleteOptions{})
	if err != nil {
		k8serr := new(k8serrors.StatusError)
		if errors.As(err, &k8serr) {
			if k8serr.Status().Code != http.StatusNotFound {
				return fmt.Errorf("failed to delete dataflow %s workflow error: %w", req.ArgoTaskID, err)
			} else {
				slog.WarnContext(ctx, "dataflow %s workflow not found for delete", slog.Any("task_id", req.ArgoTaskID))
			}
		} else {
			return fmt.Errorf("failed to delete dataflow %s workflow error: %w", req.ArgoTaskID, err)
		}
	}

	err = d.deletePVC(ctx, cluster, req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete dataflow pvc", slog.Any("task_id", req.ArgoTaskID), slog.Any("error", err))
	}

	return nil
}

// RunInformer starts workflow and pod informers for all clusters
func (d *dataflowComponentImpl) RunInformer() {
	ctx := context.Background()

	var wg sync.WaitGroup
	stopCh := make(chan struct{})
	defer close(stopCh)
	defer runtime.HandleCrash()

	clusters := d.clusterPool.GetAllCluster()
	for _, cls := range clusters {
		_, err := cls.Client.Discovery().ServerVersion()
		if err != nil {
			slog.ErrorContext(ctx, "cluster is unavailable for dataflow informer", slog.Any("cluster config", cls.CID), slog.Any("error", err))
			continue
		}

		wg.Go(func() {
			d.runWorkflowInformer(stopCh, cls)
		})
		wg.Go(func() {
			d.runPodInformer(stopCh, cls)
		})
	}
	slog.InfoContext(ctx, "dataflow informer started")
	// wait for all informers to start
	wg.Wait()
}

// runWorkflowInformer watches Argo Workflow events
func (d *dataflowComponentImpl) runWorkflowInformer(stopCh <-chan struct{}, cluster *cluster.Cluster) {
	labelSelector := fmt.Sprintf("%s=%s", types.DFLabelTagKey, types.DFLabelTagValue)
	client := cluster.ArgoClient

	f := externalversions.NewFilteredSharedInformerFactory(
		client,
		2*time.Minute,
		d.namespace,
		internalinterfaces.TweakListOptionsFunc(func(list *v1.ListOptions) {
			list.LabelSelector = labelSelector
		}),
	)

	informer := f.Argoproj().V1alpha1().Workflows().Informer()

	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			wf := obj.(*v1alpha1.Workflow)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := d.handleWorkflowEvent(ctx, wf, types.RunnerDataflowChange); err != nil {
				slog.ErrorContext(ctx, "failed to handle dataflow workflow create event",
					slog.Any("error", err), slog.String("workflow", wf.Name))
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// oldWF := oldObj.(*v1alpha1.Workflow)
			newWF := newObj.(*v1alpha1.Workflow)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := d.handleWorkflowEvent(ctx, newWF, types.RunnerDataflowChange); err != nil {
				slog.ErrorContext(ctx, "failed to handle dataflow workflow update event",
					slog.Any("error", err), slog.String("workflow", newWF.Name))
			}
		},
		DeleteFunc: func(obj interface{}) {
			wf, ok := obj.(*v1alpha1.Workflow)
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := d.handleWorkflowEvent(ctx, wf, types.RunnerDataflowDelete); err != nil {
				slog.ErrorContext(ctx, "failed to handle dataflow workflow delete event",
					slog.Any("error", err), slog.String("workflow", wf.Name))
			}

			pvcReq := &types.DataflowArgoReq{
				ArgoTaskID: wf.Name,
			}
			err := d.deletePVC(ctx, cluster, pvcReq)
			if err != nil {
				slog.ErrorContext(ctx, "failed to delete dataflow pvc due workflow delete informer event",
					slog.Any("task_id", pvcReq.ArgoTaskID), slog.Any("error", err))
			}

		},
	}

	_, err := informer.AddEventHandler(eventHandler)
	if err != nil {
		runtime.HandleError(fmt.Errorf("failed to add event handler for dataflow workflow informer: %w", err))
		return
	}

	informer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for dataflow workflow caches to sync"))
	}
}

// handleWorkflowEvent processes workflow events and reports to csghub
func (d *dataflowComponentImpl) handleWorkflowEvent(ctx context.Context, wf *v1alpha1.Workflow, eventType types.WebHookEventType) error {
	annotations := wf.Annotations
	if len(annotations) < 1 {
		return fmt.Errorf("workflow %s/%s has no annotations", wf.Namespace, wf.Name)
	}

	resID := annotations[types.DFResourceIDKey]
	if len(resID) < 1 {
		slog.WarnContext(ctx, "workflow has no resource id", slog.Any("dataflow", wf.Name))
	}
	resIDInt, err := strconv.ParseInt(resID, 10, 64)
	if err != nil {
		slog.WarnContext(ctx, "dataflow workflow has invalid resource id",
			slog.Any("wf.name", wf.Name), slog.Any("error", err), slog.String("resource_id", resID))
	}

	wfStatus := v1alpha1.WorkflowPending
	if len(wf.Status.Phase) > 0 {
		wfStatus = wf.Status.Phase
	}

	// Extract info from annotations
	wfInfo := &database.ArgoWorkflow{
		Username:     annotations[types.DFOpUserNameKey],
		UserUUID:     annotations[types.DFNSUUIDKey],
		TaskName:     annotations[types.DFJobNameKey],
		TaskId:       wf.Name,
		TaskType:     types.TaskTypeDataflow,
		ClusterID:    annotations[types.DFClusterIDKey],
		Namespace:    wf.Namespace,
		RepoType:     string(types.DatasetRepo),
		TaskDesc:     annotations[types.DFJobDescKey],
		Image:        annotations[types.DFImageKey],
		ResourceId:   resIDInt,
		ResourceName: annotations[types.DFResourceNameKey],
		Status:       wfStatus,
		Reason:       wf.Status.Message,
		QueueName:    annotations[rtypes.VolcanoAnnoQueue],
	}
	if !wf.Status.StartedAt.IsZero() {
		wfInfo.StartTime = wf.Status.StartedAt.Time
	}
	if !wf.Status.FinishedAt.IsZero() {
		wfInfo.EndTime = wf.Status.FinishedAt.Time
	}

	if len(wfInfo.TaskId) < 1 {
		return fmt.Errorf("dataflow workflow %s has no task id", wf.Name)
	}

	slog.InfoContext(ctx, "handling dataflow workflow event",
		slog.String("event_type", string(eventType)),
		slog.Any("wf", wfInfo))

	// find workflow in database or create it
	dbWF, err := d.wfStore.FindByTaskID(ctx, wfInfo.TaskId)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.WarnContext(ctx, "failed to find dataflow workflow in db",
			slog.Any("error", err), slog.String("task_id", wfInfo.TaskId))
	}
	if errors.Is(err, sql.ErrNoRows) {
		dbWF, err = d.wfStore.CreateWorkFlow(ctx, *wfInfo)
		if err != nil {
			slog.ErrorContext(ctx, "dataflow workflow failed to create in db",
				slog.Any("error", err), slog.Any("wfInfo", wfInfo))
		}
	}
	if dbWF == nil {
		slog.WarnContext(ctx, "dataflow workflow not found in db", slog.Any("wfInfo", wfInfo))
	} else {
		// Update workflow status
		dbWF.Status = wfStatus
		dbWF.Reason = wf.Status.Message
		if !wf.Status.StartedAt.IsZero() {
			dbWF.StartTime = wf.Status.StartedAt.Time
		}
		if !wf.Status.FinishedAt.IsZero() {
			dbWF.EndTime = wf.Status.FinishedAt.Time
		}
		dbWF, err := d.wfStore.UpdateWorkFlow(ctx, *dbWF)
		if err != nil {
			slog.ErrorContext(ctx, "failed to update dataflow workflow in db",
				slog.Any("error", err), slog.Any("dbWF", dbWF))
		}
	}

	// Report event to csghub
	d.reportDataflowEvent(ctx, wfInfo.ClusterID, wfInfo, eventType)

	return nil
}

// runPodInformer watches Pod events for dataflow workloads
func (d *dataflowComponentImpl) runPodInformer(stopCh <-chan struct{}, cluster *cluster.Cluster) {
	labelSelector := fmt.Sprintf("%s=%s", types.DFLabelTagKey, types.DFLabelTagValue)

	factory := informers.NewSharedInformerFactoryWithOptions(
		cluster.Client,
		1*time.Hour,
		informers.WithNamespace(d.namespace),
		informers.WithTweakListOptions(func(options *v1.ListOptions) {
			options.LabelSelector = labelSelector
		}),
	)

	podInformer := factory.Core().V1().Pods()

	_, err := podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := d.handlePodEvent(ctx, pod, types.RunnerDataflowPodUpdate); err != nil {
				slog.ErrorContext(ctx, "failed to handle dataflow pod add event",
					slog.Any("error", err), slog.String("pod", pod.Name))
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// oldPod := oldObj.(*corev1.Pod)
			newPod, ok := newObj.(*corev1.Pod)
			if !ok {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := d.handlePodEvent(ctx, newPod, types.RunnerDataflowPodUpdate); err != nil {
				slog.ErrorContext(ctx, "failed to handle dataflow pod update event",
					slog.Any("error", err), slog.String("pod", newPod.Name))
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := d.handlePodEvent(ctx, pod, types.RunnerDataflowPodDelete); err != nil {
				slog.ErrorContext(ctx, "failed to handle dataflow pod delete event",
					slog.Any("error", err), slog.String("pod", pod.Name))
			}
		},
	})

	if err != nil {
		runtime.HandleError(fmt.Errorf("failed to add event handler for dataflow pod informer: %w", err))
		return
	}

	factory.Start(stopCh)

	if !cache.WaitForCacheSync(stopCh, podInformer.Informer().HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for dataflow pod caches to sync"))
	}
}

// handlePodEvent processes pod events and reports to csghub
func (d *dataflowComponentImpl) handlePodEvent(ctx context.Context, pod *corev1.Pod, eventType types.WebHookEventType) error {
	annotations := pod.Annotations
	if len(annotations) < 1 {
		return fmt.Errorf("dataflow pod %s/%s has no annotations", pod.Namespace, pod.Name)
	}

	// Extract info from annotations
	clusterID := annotations[types.DFClusterIDKey]
	taskID := annotations[types.DFArgoTaskIDKey]
	dagTaskID := pod.Labels[types.DFLabelDagTaskIDKey]
	dagTaskName := pod.Labels[types.DFLabelDagTaskNameKey]
	if len(dagTaskID) < 1 {
		return fmt.Errorf("dataflow pod %s/%s has no dag task id", pod.Namespace, pod.Name)
	}

	dagTask := types.DataflowDagTask{
		Name:   dagTaskName,
		Status: string(pod.Status.Phase),
	}

	if !pod.Status.StartTime.IsZero() {
		dagTask.StartTime = pod.Status.StartTime.Format("2006-01-02 15:04:05.000")
	}

	if pod.Status.Phase != corev1.PodPending && pod.Status.Phase != corev1.PodRunning {
		dagTask.EndTime = time.Now().Format("2006-01-02 15:04:05.000")
	}

	podMap := make(map[string]types.DataflowDagTask)
	podMap[dagTaskID] = dagTask

	jsonStr, err := json.Marshal(podMap)
	if err != nil {
		return fmt.Errorf("failed to marshal dag_tasks pod map: %w", err)
	}

	wfInfo := &database.ArgoWorkflow{
		TaskId:      taskID,
		ClusterNode: pod.Spec.NodeName,
		DagTasks:    string(jsonStr),
	}

	slog.InfoContext(ctx, "handling dataflow pod event", slog.Any("wfInfo", wfInfo))

	// Report event to csghub
	d.reportDataflowEvent(ctx, clusterID, wfInfo, eventType)
	return nil
}

// reportDataflowEvent sends event to csghub API
func (d *dataflowComponentImpl) reportDataflowEvent(ctx context.Context, clusterID string, wf *database.ArgoWorkflow, eventType types.WebHookEventType) {
	event := &types.WebHookSendEvent{
		WebHookHeader: types.WebHookHeader{
			EventType: eventType,
			EventTime: time.Now().Unix(),
			ClusterID: clusterID,
			DataType:  types.WebHookDataTypeObject,
		},
		Data: wf,
	}

	slog.InfoContext(ctx, "reporting dataflow event", slog.Any("event", event))

	go func() {
		err := common.Push(d.config.Runner.WebHookEndpoint, d.config.APIToken, event)
		if err != nil {
			slog.ErrorContext(ctx, "failed to push dataflow workflow event", slog.Any("error", err))
		}
	}()
}
