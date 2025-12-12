package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"opencsg.com/csghub-server/component/reporter"

	v1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	ctypes "opencsg.com/csghub-server/common/types"
	embed "opencsg.com/csghub-server/docker/spaces/builder"
	rcommon "opencsg.com/csghub-server/runner/common"
	"opencsg.com/csghub-server/runner/types"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

const (
	WORK_META_DATA = "work_meta_data"
)

var (
	kanikoCachePVC string = "imagebuilder-pvc-kaniko-cache"

	initContainerType  string = "init-repo"
	buildContainerType string = "build"
)

type ImagebuilderComponent interface {
	Build(ctx context.Context, req ctypes.ImageBuilderRequest) error
	Stop(ctx context.Context, req ctypes.ImageBuildStopReq) error
}

type imagebuilderComponentImpl struct {
	config      *config.Config
	clusterPool *cluster.ClusterPool
	logReporter reporter.LogCollector
}

func NewImagebuilderComponent(ctx context.Context,
	config *config.Config,
	clusterPool *cluster.ClusterPool,
	logReporter reporter.LogCollector) (ImagebuilderComponent, error) {
	ibc := &imagebuilderComponentImpl{
		config:      config,
		clusterPool: clusterPool,
		logReporter: logReporter,
	}

	if err := workFlowInit(ctx, config, clusterPool); err != nil {
		slog.ErrorContext(ctx, "failed to init workflow", slog.Any("error", err))
		return nil, err
	}

	go ibc.runInformer(ctx)
	return ibc, nil
}

func (ibc *imagebuilderComponentImpl) GetCluster(ctx context.Context, clusterId string) (*cluster.Cluster, error) {
	return ibc.clusterPool.GetClusterByID(ctx, clusterId)
}

func (ibc *imagebuilderComponentImpl) Build(ctx context.Context, req ctypes.ImageBuilderRequest) error {
	ibc.pushLog(req.DeployId, strconv.FormatInt(req.TaskId, 10), ctypes.StagePreBuild, ctypes.StepInitializing, "start to build image workflow")
	namespace := ibc.config.Cluster.SpaceNamespace
	imagePath := path.Join(ibc.config.Space.DockerRegBase, req.LastCommitID)
	cluster, err := ibc.GetCluster(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get cluster by id: %w", err)
	}

	cInfo, err := ibc.clusterPool.ClusterStore.ByClusterConfig(ctx, cluster.CID)
	if err != nil {
		return fmt.Errorf("failed to get cluster by config: %w", err)
	}

	if len(strings.TrimSpace(cInfo.StorageClass)) > 0 {
		err = ibc.newPersistentVolumeClaim(ctx, cluster, kanikoCachePVC)
		if err != nil {
			return fmt.Errorf("failed to create pvc: %w", err)
		}
	}

	workMeta, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal imagebuilder work: %w", err)
	}

	createWorkflowName := ibc.generateWorkName(req.OrgName, req.SpaceName, req.DeployId, fmt.Sprintf("%d", req.TaskId))

	wft, err := wfTemplateForImageBuilder(ibc.config, req, imagePath, cInfo.StorageClass, createWorkflowName)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create imagebuilder workflow template", "err", err)
		return fmt.Errorf("failed to create imagebuilder workflow template: %w", err)
	}

	wft.Annotations = map[string]string{
		WORK_META_DATA: string(workMeta),
	}

	// clear old workflow if exist with same name
	err = ibc.checkAndRemoveExistingWorkflow(ctx, cluster, namespace, createWorkflowName)
	if err != nil {
		return fmt.Errorf("failed to check and remove existing workflow %s/%s: %w", namespace, createWorkflowName, err)
	}

	// create new workflow for image builder
	_, err = cluster.ArgoClient.ArgoprojV1alpha1().Workflows(namespace).Create(ctx, wft, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create imagebuilder workflow %s/%s in argo: %w", namespace, createWorkflowName, err)
	}

	return nil
}

func (ibc *imagebuilderComponentImpl) checkAndRemoveExistingWorkflow(ctx context.Context, cluster *cluster.Cluster,
	namespace, createWorkflowName string) error {
	checkWft, err := cluster.ArgoClient.ArgoprojV1alpha1().Workflows(namespace).Get(ctx, createWorkflowName, metav1.GetOptions{})
	slog.Debug("get workflow for space image build", slog.Any("checkWft", checkWft), slog.Any("error", err))
	if err != nil {
		if statusErr, ok := err.(*k8serrors.StatusError); ok {
			//{Status: "Failure", Message: "workflows.argoproj.io \"xxxx\" not found", Reason: "NotFound", Code: 404}}
			if statusErr.Status().Code == 404 {
				// no old workflow found
				return nil
			}
		}
		return fmt.Errorf("failed to check if workflow exist by name %s/%s, error: %w", namespace, createWorkflowName, err)
	}

	if checkWft != nil {
		// remove existing workflow
		err = cluster.ArgoClient.ArgoprojV1alpha1().Workflows(namespace).Delete(ctx, createWorkflowName, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to clean up the existing workflow %s/%s for create new one, error: %w", namespace, createWorkflowName, err)
		}
		time.Sleep(time.Second * 2)
	}

	return nil
}

func (ibc *imagebuilderComponentImpl) Stop(ctx context.Context, req ctypes.ImageBuildStopReq) error {
	cluster, err := ibc.GetCluster(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to imagebuilder get cluster by id: %w", err)
	}
	wfName := ibc.generateWorkName(req.OrgName, req.SpaceName, req.DeployId, req.TaskId)
	err = cluster.ArgoClient.ArgoprojV1alpha1().Workflows(ibc.config.Cluster.SpaceNamespace).Delete(ctx, wfName, metav1.DeleteOptions{})
	return err
}

func workFlowInit(ctx context.Context, config *config.Config, clusterPool *cluster.ClusterPool) error {
	namespace := config.Cluster.SpaceNamespace

	fcMap := make(map[string][]byte)
	for _, cfg := range types.ConfigMapFiles {
		data, err := embed.ImagebuilderFs.ReadFile(cfg.FileName)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", cfg.FileName, err)
		}
		fcMap[cfg.FileName] = data
	}

	for _, cluster := range clusterPool.Clusters {
		for _, cfg := range types.ConfigMapFiles {
			data, exists := fcMap[cfg.FileName]
			if !exists {
				return fmt.Errorf("file %s not loaded", cfg.FileName)
			}
			cmd := types.CMConfig{
				Namespace:   namespace,
				CmName:      cfg.ConfigMapName,
				DataKey:     cfg.FileName,
				FileContent: data,
			}
			if err := createOrUpdateConfigMap(ctx, cluster.Client, cmd); err != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("failed to create %s configmap", cfg.FileName), "err", err)
				continue
			}

		}
	}

	return nil
}

func (ibc *imagebuilderComponentImpl) runInformer(ctx context.Context) {
	for _, cls := range ibc.clusterPool.Clusters {
		go func(cluster *cluster.Cluster) {
			ibc.workInformer(ctx, cluster)
		}(cls)
	}
}

func (ibc *imagebuilderComponentImpl) workInformer(ctx context.Context, cluster *cluster.Cluster) {
	labelSelector := "workflow-scope=imagebuilder"

	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			wf := obj.(*v1alpha1.Workflow)
			err := ibc.updateImagebuilderWork(ctx, cluster, wf)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				slog.ErrorContext(ctx, "fail to add imagebuilder task", slog.Any("error", err), slog.Any("work_name", wf.Name))
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newWF := newObj.(*v1alpha1.Workflow)
			err := ibc.updateImagebuilderWork(ctx, cluster, newWF)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				slog.ErrorContext(ctx, "fail to update imagebuilder task", slog.Any("error", err), slog.Any("work_name", newWF.Name))
			}
		},
		DeleteFunc: func(obj interface{}) {
			wf := obj.(*v1alpha1.Workflow)
			err := ibc.updateImagebuilderWork(ctx, cluster, wf)
			if err != nil {
				slog.ErrorContext(ctx, "fail to delete imagebuilder task", slog.Any("error", err), slog.Any("work_name", wf.Name))
			}
		},
	}

	CreateInfomerFactory(cluster.ArgoClient, ibc.config.Cluster.SpaceNamespace, labelSelector, eventHandler)
}

func wfTemplateForImageBuilder(cfg *config.Config, params ctypes.ImageBuilderRequest, imagePath, storeClass, workName string) (*v1alpha1.Workflow, error) {
	labels := map[string]string{
		"workflow-scope":             "imagebuilder",
		ctypes.LogLabelTypeKey:       ctypes.LogLabelImageBuilder,
		ctypes.StreamKeyDeployID:     params.DeployId,
		ctypes.StreamKeyDeployTaskID: strconv.FormatInt(params.TaskId, 10),
	}

	builderArgs := []string{
		"--context=/shared/" + params.SpaceName,
		"--destination=" + imagePath,
		"--build-arg=GIT_IMAGE=" + cfg.Runner.ImageBuilderGitImage,
	}

	for _, arg := range cfg.Runner.ImageBuilderKanikoArgs {
		if arg == "" || strings.HasPrefix(arg, "--context") || strings.HasPrefix(arg, "--destination") {
			continue
		}
		builderArgs = append(builderArgs, arg)
	}

	// volumes
	specVolumes := []corev1.Volume{}
	specVolumes = append(specVolumes, corev1.Volume{
		Name: "shared",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	for _, cfg := range types.ConfigMapFiles {
		specVolumes = append(specVolumes, corev1.Volume{
			Name: cfg.VolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cfg.ConfigMapName,
					},
				},
			},
		})
	}

	sharedVolumeMount := corev1.VolumeMount{
		Name:      "shared",
		MountPath: "/shared",
	}

	// init container volume mounts
	initContainerVolumeMounts := []corev1.VolumeMount{}
	initContainerVolumeMounts = append(initContainerVolumeMounts, sharedVolumeMount)

	for _, cfg := range types.ConfigMapFiles {
		initContainerVolumeMounts = append(initContainerVolumeMounts, corev1.VolumeMount{
			Name:      cfg.VolumeName,
			MountPath: "/builder/config/" + cfg.FileName,
			SubPath:   cfg.FileName,
			ReadOnly:  cfg.ReadOnly,
		})
	}

	// container volume mounts
	containerVolumeMounts := []corev1.VolumeMount{}
	containerVolumeMounts = append(containerVolumeMounts, sharedVolumeMount, corev1.VolumeMount{
		Name:      "docker-config",
		MountPath: "/kaniko/.docker",
	})

	if storeClass != "" {
		builderArgs = append(builderArgs, "--cache=true")
		builderArgs = append(builderArgs, "--cache-dir=/kaniko-cache")

		specVolumes = append(specVolumes, corev1.Volume{
			Name: "kaniko-cache",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: kanikoCachePVC,
				},
			},
		})

		containerVolumeMounts = append(containerVolumeMounts, corev1.VolumeMount{
			Name:      "kaniko-cache",
			MountPath: "/kaniko-cache",
		})
	}

	wfTemplate := &v1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "imagebuilder-",
			Name:         workName,
			Labels:       labels,
		},
		Spec: v1alpha1.WorkflowSpec{
			PodMetadata: &v1alpha1.Metadata{
				Labels: labels,
			},
			Entrypoint: "main",
			Volumes:    specVolumes,
			Templates: []v1alpha1.Template{
				{
					Name: "main",
					Steps: []v1alpha1.ParallelSteps{
						{
							Steps: []v1alpha1.WorkflowStep{
								{
									Name:     "imagebuilder",
									Template: buildContainerType,
								},
							},
						},
					},
				},
				{
					Name: buildContainerType,
					InitContainers: []v1alpha1.UserContainer{
						{
							Container: corev1.Container{
								Name:    initContainerType,
								Image:   cfg.Runner.ImageBuilderGitImage,
								Command: []string{"sh", "-c"},
								Args: []string{
									`sh /builder/config/init.sh  $REPO $REPO_NAME $USER_NAME $SECRET $SPACE_URL $GIT_REF $SDK $PYTHON_VERSION $HARDWARE $DRIVER_VERSION && cp -r $REPO/$REPO_NAME/ /shared`,
								},
								SecurityContext: &corev1.SecurityContext{
									RunAsUser: ptr.To(int64(0)),
								},
								Env: []corev1.EnvVar{
									{Name: "REPO", Value: "/builder"},
									{Name: "REPO_NAME", Value: params.SpaceName},
									{Name: "USER_NAME", Value: params.UserId},
									{Name: "SECRET", Value: params.GitAccessToken},
									{Name: "SPACE_URL", Value: params.SpaceURL},
									{Name: "GIT_REF", Value: params.GitRef},
									{Name: "SDK", Value: params.Sdk},
									{Name: "PYTHON_VERSION", Value: params.PythonVersion},
									{Name: "HARDWARE", Value: params.Hardware},
									{Name: "DRIVER_VERSION", Value: params.DriverVersion},
								},
								VolumeMounts: initContainerVolumeMounts,
							},
						},
					},
					Container: &corev1.Container{
						Name:         buildContainerType,
						Image:        cfg.Runner.ImageBuilderKanikoImage,
						Args:         builderArgs,
						VolumeMounts: containerVolumeMounts,
						Env: []corev1.EnvVar{
							{Name: "REPO_NAME", Value: params.SpaceName},
						},
					},
				},
			},
		},
	}

	wfTemplate.Spec.TTLStrategy = &v1alpha1.TTLStrategy{
		SecondsAfterSuccess:    ptr.To(int32(cfg.Runner.ImageBuilderJobTTL)),
		SecondsAfterFailure:    ptr.To(int32(cfg.Runner.ImageBuilderJobTTL)),
		SecondsAfterCompletion: ptr.To(int32(cfg.Runner.ImageBuilderJobTTL)),
	}

	wfTemplate.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
		{
			Name: cfg.Space.ImagePullSecret,
		},
	}

	wfTemplate.Spec.Volumes = append(wfTemplate.Spec.Volumes, corev1.Volume{
		Name: "docker-config",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: cfg.Space.ImagePullSecret,
				Items: []corev1.KeyToPath{
					{
						Key:  ".dockerconfigjson",
						Path: "config.json",
					},
				},
			},
		},
	})
	wfTemplate.Spec.ServiceAccountName = cfg.Argo.ServiceAccountName

	return wfTemplate, nil
}

func (ibc *imagebuilderComponentImpl) updateImagebuilderWork(ctx context.Context, cluster *cluster.Cluster, wf *v1alpha1.Workflow) error {
	workMeta, err := ibc.workMeataDataFromWF(wf)
	if err != nil {
		return fmt.Errorf("failed to get work meta data from wf: %w", err)
	}

	ibc.addKServiceWithEvent(ctypes.RunnerBuilderChange, ctypes.ImageBuilderEvent{
		DeployId:   workMeta.DeployId,
		TaskId:     workMeta.TaskId,
		Status:     string(wf.Status.Phase),
		Message:    wf.Status.Message,
		ImagetPath: workMeta.LastCommitID,
	})

	return err
}

func createOrUpdateConfigMap(ctx context.Context, client kubernetes.Interface, cmc types.CMConfig) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: cmc.CmName,
		},
		Data: map[string]string{
			cmc.DataKey: string(cmc.FileContent),
		},
	}

	existingCM, err := client.CoreV1().ConfigMaps(cmc.Namespace).Get(ctx, cmc.CmName, metav1.GetOptions{})
	if err == nil {
		// update ConfigMap
		existingCM.Data[cmc.DataKey] = string(cmc.FileContent)
		_, err := client.CoreV1().ConfigMaps(cmc.Namespace).Update(ctx, existingCM, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("%s ConfigMap update failed: %w", cmc.CmName, err)
		}
	} else {
		// create ConfigMap
		_, err := client.CoreV1().ConfigMaps(cmc.Namespace).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("%s ConfigMap creation failed: %w", cmc.CmName, err)
		}
	}

	return nil
}

func (ibc *imagebuilderComponentImpl) newPersistentVolumeClaim(ctx context.Context, cluster *cluster.Cluster, pvcName string) error {
	// Check if it already exists
	slog.Info("check pvc for imagebuilder", slog.String("pvc", pvcName), slog.String("storageClass", cluster.StorageClass), slog.Any("storage len", len(cluster.StorageClass)))
	_, err := cluster.Client.CoreV1().PersistentVolumeClaims(ibc.config.Cluster.SpaceNamespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	slog.Info("create pvc for imagebuilder", slog.String("pvc", pvcName), slog.String("storageClass", cluster.StorageClass), slog.Any("storage len", len(cluster.StorageClass)))
	storage, err := resource.ParseQuantity("50Gi")
	if err != nil {
		return err
	}

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: ibc.config.Cluster.SpaceNamespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storage,
				},
			},
			StorageClassName: &cluster.StorageClass,
		},
	}
	_, err = cluster.Client.CoreV1().PersistentVolumeClaims(ibc.config.Cluster.SpaceNamespace).Create(ctx, &pvc, metav1.CreateOptions{})
	return err
}

func (ibc *imagebuilderComponentImpl) workMeataDataFromWF(wf *v1alpha1.Workflow) (*ctypes.ImageBuilderRequest, error) {
	meta := wf.Annotations[WORK_META_DATA]
	var workMeta ctypes.ImageBuilderRequest
	err := json.Unmarshal([]byte(meta), &workMeta)
	if err != nil {
		return nil, err
	}
	return &workMeta, nil
}

func (ibc *imagebuilderComponentImpl) pushLog(deployId string, taskId string, stage ctypes.Stage, step ctypes.Step, log string) {
	entry := ctypes.LogEntry{
		Message:  log,
		Stage:    stage,
		Step:     step,
		DeployID: deployId,
		Labels: map[string]string{
			ctypes.LogLabelTypeKey:       ctypes.LogLabelImageBuilder,
			ctypes.StreamKeyDeployID:     deployId,
			ctypes.StreamKeyDeployTaskID: taskId,
		},
		PodInfo: &ctypes.PodInfo{},
	}
	ibc.logReporter.Report(entry)
}

func (ibc *imagebuilderComponentImpl) addKServiceWithEvent(eventType ctypes.WebHookEventType, data ctypes.ImageBuilderEvent) {
	event := &ctypes.WebHookSendEvent{
		WebHookHeader: ctypes.WebHookHeader{
			EventType: eventType,
			EventTime: time.Now().Unix(),
			DataType:  ctypes.WebHookDataTypeObject,
		},
		Data: data,
	}

	go func() {
		err := rcommon.Push(ibc.config.Runner.WebHookEndpoint, ibc.config.APIToken, event)
		if err != nil {
			slog.Error("failed to push imagebuilder service status event", slog.Any("error", err))
		}
	}()
}

func (ibc *imagebuilderComponentImpl) generateWorkName(orgName, spaceName, deployId, taskID string) string {
	if orgName == "" {
		orgName = "default"
	}
	if spaceName == "" {
		spaceName = "default"
	}

	clean := func(s string) string {
		// Convert to lowercase
		s = strings.ToLower(s)
		// Replace non-allowed characters with hyphens
		re := regexp.MustCompile(`[^a-z0-9-]`)
		s = re.ReplaceAllString(s, "-")
		// Merge consecutive hyphens
		re = regexp.MustCompile(`-+`)
		s = re.ReplaceAllString(s, "-")
		// Remove leading and trailing hyphens
		return strings.Trim(s, "-")
	}

	orgName = clean(orgName)
	spaceName = clean(spaceName)
	if len(orgName) > 8 {
		orgName = orgName[:8]
	}

	if len(spaceName) > 8 {
		spaceName = spaceName[:8]
	}

	baseName := fmt.Sprintf("sib-%s-%s-%s-%s", orgName, spaceName, deployId, taskID)
	if len(baseName) > 63 {
		baseName = baseName[:63]
	}
	return baseName
}
