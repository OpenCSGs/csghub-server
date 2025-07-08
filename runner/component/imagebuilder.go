package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	v1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	embed "opencsg.com/csghub-server/docker/spaces/builder"
	rcommon "opencsg.com/csghub-server/runner/common"
	"opencsg.com/csghub-server/runner/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

var (
	workCreateStatus   string = "Create"
	initContainerType  string = "init-repo"
	buildContainerType string = "build"

	containerStatusUnknown    string = "Unknown"
	containerStatusWaiting    string = "Waiting"
	containerStatusRunning    string = "Running"
	containerStatusTerminated string = "Terminated"
)

type ImagebuilderComponent interface {
	Build(ctx context.Context, spaceConfig types.SpaceBuilderConfig) (*types.ImageBuilderWork, error)
	Status(ctx context.Context, buildId string) (*types.ImageBuilderWork, error)
	Logs(ctx context.Context, buildId string) (chan []byte, error)
}

type imagebuilderComponentImpl struct {
	config      *config.Config
	clusterPool *cluster.ClusterPool
	db          database.ImageBuilderWorkStore
}

func NewImagebuilderComponent(ctx context.Context, config *config.Config, clusterPool *cluster.ClusterPool) (ImagebuilderComponent, error) {
	ibc := &imagebuilderComponentImpl{
		config:      config,
		clusterPool: clusterPool,
		db:          database.NewImageBuilderStore(),
	}

	if err := ibc.workFlowInit(ctx); err != nil {
		slog.Error("failed to init workflow", slog.Any("error", err))
		return nil, err
	}
	go ibc.workInformer(ctx)
	return ibc, nil
}

func (ibc *imagebuilderComponentImpl) GetCluster(ctx context.Context, clusterId string) (*cluster.Cluster, error) {
	return ibc.clusterPool.GetClusterByID(ctx, clusterId)
}

func (ibc *imagebuilderComponentImpl) Build(ctx context.Context, spaceConfig types.SpaceBuilderConfig) (*types.ImageBuilderWork, error) {
	namespace := ibc.config.Runner.ImageBuilderNamespace
	cluster, err := ibc.GetCluster(ctx, ibc.config.Runner.ImageBuilderClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster by id: %w", err)
	}

	imageName := buildImageName(spaceConfig.OrgName, spaceConfig.SpaceName, spaceConfig.BuildId)
	imagePath := joinImagePath(ibc.config.Space.DockerRegBase, imageName)
	wft, err := wfTemplateForImageBuilder(
		ibc.config.Runner.ImageBuilderGitImage,
		ibc.config.Runner.ImageBuilderKanikoImage,
		imagePath,
		spaceConfig,
		ibc.config.Runner.ImageBuilderKanikoArgs,
	)

	if err != nil {
		slog.Error("failed to create imagebuilder workflow template", "err", err)
		return nil, fmt.Errorf("failed to create imagebuilder workflow template: %w", err)
	}

	wft.Spec.TTLStrategy = &v1alpha1.TTLStrategy{
		SecondsAfterSuccess:    ptr.To(int32(ibc.config.Runner.ImageBuilderJobTTL)),
		SecondsAfterFailure:    ptr.To(int32(ibc.config.Runner.ImageBuilderJobTTL)),
		SecondsAfterCompletion: ptr.To(int32(ibc.config.Runner.ImageBuilderJobTTL)),
	}

	wft.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
		{
			Name: ibc.config.Space.ImagePullSecret,
		},
	}

	wft.Spec.Volumes = append(wft.Spec.Volumes, corev1.Volume{
		Name: "docker-config",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: ibc.config.Space.ImagePullSecret,
				Items: []corev1.KeyToPath{
					{
						Key:  ".dockerconfigjson",
						Path: "config.json",
					},
				},
			},
		},
	})
	wft.Spec.ServiceAccountName = ibc.config.Argo.ServiceAccountName

	wf, err := cluster.ArgoClient.ArgoprojV1alpha1().Workflows(namespace).Create(ctx, wft, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create imagebuilder workflow in argo: %w", err)
	}

	ibm := &database.ImageBuilderWork{
		WorkName:            wf.Name,
		WorkStatus:          workCreateStatus,
		ImagePath:           imageName,
		BuildId:             types.JointSpaceNameBuildId(spaceConfig.OrgName, spaceConfig.SpaceName, spaceConfig.BuildId),
		Namespace:           namespace,
		ClusterID:           ibc.config.Runner.ImageBuilderClusterID,
		InitContainerStatus: containerStatusUnknown,
	}
	if _, err := ibc.db.CreateOrUpdateByBuildID(ctx, ibm); err != nil {
		return nil, fmt.Errorf("failed to create imagebuilder in db: %w", err)
	}

	return &types.ImageBuilderWork{
		WorkName:   ibm.WorkName,
		WorkStatus: ibm.WorkStatus,
		Message:    ibm.Message,
		ImagePath:  ibm.ImagePath,
		BuildId:    ibm.BuildId,
	}, nil
}

func (ibc *imagebuilderComponentImpl) Status(ctx context.Context, buildId string) (*types.ImageBuilderWork, error) {
	ibw, err := ibc.db.QueryStatusByBuildID(ctx, buildId)
	if err != nil {
		return nil, err
	}

	// check timeout with workflow status at unknown, running, pending
	if ibw.WorkStatus == string(v1alpha1.WorkflowUnknown) ||
		ibw.WorkStatus == string(v1alpha1.WorkflowRunning) ||
		ibw.WorkStatus == string(v1alpha1.WorkflowPending) {

		if time.Since(ibw.CreatedAt) > time.Duration(ibc.config.Runner.ImageBuilderStatusTTL)*time.Second {
			cluster, err := ibc.GetCluster(ctx, ibw.ClusterID)
			if err != nil {
				return nil, fmt.Errorf("failed to imagebuilder get cluster by id: %w", err)
			}
			// get workflow status for cluster
			wf, err := cluster.ArgoClient.ArgoprojV1alpha1().Workflows(ibw.Namespace).Get(ctx, ibw.WorkName, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to imagebuilder get workflow by name: %w", err)

			}
			// change status and get logs
			err = ibc.updateImagebuilderWork(ctx, cluster, wf)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
				} else {
					slog.Error("failed to imagebuilder update workflow", slog.Any("error", err))
				}
			}
			ibw, err = ibc.db.QueryStatusByBuildID(ctx, ibw.BuildId)
			if err != nil {
				return nil, fmt.Errorf("failed to imagebuilder query status by build id: %w", err)
			}
		}
	}

	return &types.ImageBuilderWork{
		WorkName:   ibw.WorkName,
		WorkStatus: ibw.WorkStatus,
		Message:    ibw.Message,
		ImagePath:  ibw.ImagePath,
		BuildId:    ibw.BuildId,
	}, nil
}

func (ibc *imagebuilderComponentImpl) Logs(ctx context.Context, buildId string) (chan []byte, error) {
	ch := make(chan []byte)
	go func() {
		defer close(ch)
		ibw, err := ibc.db.QueryByBuildID(ctx, buildId)
		if err != nil {
			ch <- []byte(fmt.Sprintf("failed to query imagebuilder work by work name: error: %s", err.Error()))
			return
		}

		if ibw.WorkStatus == string(v1alpha1.WorkflowUnknown) {
			ch <- []byte(fmt.Sprintf("imagebuilder workflow %s is not running status: %s", ibw.WorkName, ibw.WorkStatus))
			return
		}

		// get logs from cluster
		cluster, err := ibc.GetCluster(ctx, ibw.ClusterID)
		if err != nil {
			ch <- []byte(fmt.Sprintf("failed to get cluster by id: %s", err.Error()))
			return
		}

		initLogs := ibc.getInitContainerLogsStream(ctx, cluster, ibw)
		for log := range initLogs {
			ch <- []byte(log)
		}
		buildLogs := ibc.getMainContainerLogsStream(ctx, cluster, ibw)
		for log := range buildLogs {
			ch <- []byte(log)
		}
	}()
	return ch, nil
}

func (ibc *imagebuilderComponentImpl) workFlowInit(ctx context.Context) error {
	namespace := ibc.config.Runner.ImageBuilderNamespace

	fcMap := make(map[string][]byte)
	for _, cfg := range types.ConfigMapFiles {
		data, err := embed.ImagebuilderFs.ReadFile(cfg.FileName)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", cfg.FileName, err)
		}
		fcMap[cfg.FileName] = data
	}

	for _, cluster := range ibc.clusterPool.Clusters {
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
				slog.Error(fmt.Sprintf("failed to create %s configmap", cfg.FileName), "err", err)
				continue
			}

		}
	}

	return nil
}

func (ibc *imagebuilderComponentImpl) workInformer(ctx context.Context) {
	cluster, err := ibc.GetCluster(ctx, ibc.config.Runner.ImageBuilderClusterID)
	if err != nil {
		slog.Error("failed to get cluster for image builder", "clusterID", ibc.config.Runner.ImageBuilderClusterID, "error", err)
		return
	}
	labelSelector := "workflow-scope=imagebuilder"

	eventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			wf := obj.(*v1alpha1.Workflow)
			err := ibc.updateImagebuilderWork(ctx, cluster, wf)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				slog.Error("fail to update imagebuilder task", slog.Any("error", err), slog.Any("work_name", wf.Name))
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newWF := newObj.(*v1alpha1.Workflow)
			// compare status
			if newWF.Status.Nodes == nil {
				return
			}
			err := ibc.updateImagebuilderWork(ctx, cluster, newWF)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				slog.Error("fail to update imagebuilder task", slog.Any("error", err), slog.Any("work_name", newWF.Name))
			}
		},
		DeleteFunc: func(obj interface{}) {
			wf := obj.(*v1alpha1.Workflow)
			slog.Info("delete imagebuilder task", slog.Any("work_name", wf.Name), slog.Any("status", wf.Status.Phase))
		},
	}

	CreateInfomerFactory(cluster.ArgoClient, ibc.config.Runner.ImageBuilderNamespace, labelSelector, eventHandler)
}

func wfTemplateForImageBuilder(gitImg, kanikoImg, imageDestination string, params types.SpaceBuilderConfig, kanikoArgs []string) (*v1alpha1.Workflow, error) {
	builderArgs := []string{
		"--context=/shared/" + params.SpaceName,
		"--destination=" + imageDestination,
		"--build-arg=GIT_IMAGE=" + gitImg,
	}
	for _, arg := range kanikoArgs {
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

	return &v1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "imagebuilder-",
			Labels: map[string]string{
				"workflow-scope": "imagebuilder",
			},
		},
		Spec: v1alpha1.WorkflowSpec{
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
								Image:   gitImg,
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
						Image:        kanikoImg,
						Args:         builderArgs,
						VolumeMounts: containerVolumeMounts,
					},
				},
			},
		},
	}, nil
}

func (ibc *imagebuilderComponentImpl) updateImagebuilderWork(ctx context.Context, cluster *cluster.Cluster, wf *v1alpha1.Workflow) error {
	if wf.Status.Nodes == nil {
		return nil
	}

	// get imagebuilder work from db
	ibw, err := ibc.db.FindByWorkName(ctx, wf.Name)
	if err != nil {
		return fmt.Errorf("failed to get wf [%s] from db: %w", wf.Name, err)
	}

	podName := ibc.getPodNameFromWorkflow(wf)
	if ibw.PodName == "" {
		ibw.PodName = podName
	}

	if ibw.WorkStatus != string(wf.Status.Phase) {
		ibw.WorkStatus = string(wf.Status.Phase)
		ibw.Message = wf.Status.Message
	}

	// get container status and logs
	pod, err := cluster.Client.CoreV1().Pods(ibw.Namespace).Get(ctx, ibw.PodName, metav1.GetOptions{})
	if err != nil {
		slog.Warn("failed to get pod", "pod", ibw.PodName, "error", err)
	} else {
		ibw.InitContainerStatus = getInitContainerStatus(pod, initContainerType)
		// get logs from cluster
		if ibw.InitContainerStatus == containerStatusTerminated {
			logs, err := getContainerLogs(ctx, cluster, pod, initContainerType)
			if err != nil {
				slog.Warn("failed to get pod log", "pod", ibw.PodName, "error", err)
			} else {
				ibw.InitContainerLog = string(logs)
			}
		}
		if ibw.WorkStatus != string(v1alpha1.WorkflowRunning) && ibw.WorkStatus != string(v1alpha1.WorkflowPending) && ibw.WorkStatus != string(v1alpha1.WorkflowUnknown) {
			logs, err := getContainerLogs(ctx, cluster, pod, buildContainerType)
			if err != nil {
				slog.Warn("failed to get main container logs from cluster", "err:", err.Error())
			} else {
				ibw.MainContainerLog = string(logs)
			}
		}
	}

	if _, err = ibc.db.UpdateByWorkName(ctx, ibw); err != nil {
		return fmt.Errorf("failed to update imagebuilder in db: %w", err)
	}

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

func buildImageName(orgName, spaceName, buildId string) string {
	// Example: opencsg_test-model:1-1678901234
	timestamp := time.Now().Unix()
	name := orgName + "_" + spaceName + ":" + buildId + "-" + fmt.Sprintf("%d", timestamp)
	imageName := strings.ToLower(name)
	return imageName
}

func joinImagePath(base, imageName string) string {
	return path.Join(base, imageName)
}

func (ibc *imagebuilderComponentImpl) constructPodName(wfName, templateName, nodeID string) string {
	parts := strings.Split(nodeID, "-")
	suffix := parts[len(parts)-1]
	return fmt.Sprintf("%s-%s-%s", wfName, templateName, suffix)
}

func (ibc *imagebuilderComponentImpl) getPodNameFromWorkflow(wf *v1alpha1.Workflow) string {
	var podName string
	// get pod name
	for _, node := range wf.Status.Nodes {
		if node.Type == v1alpha1.NodeTypePod {
			podName = ibc.constructPodName(wf.Name, node.TemplateName, node.ID)
			break
		}
	}

	return podName
}

func getInitContainerStatus(pod *corev1.Pod, containerName string) string {
	var status string = containerStatusUnknown
	for _, container := range pod.Status.InitContainerStatuses {
		if container.Name == containerName {
			if container.State.Waiting != nil {
				status = containerStatusWaiting
			}
			if container.State.Running != nil {
				status = containerStatusRunning
			}
			if container.State.Terminated != nil {
				status = containerStatusTerminated
			}
			break
		}
	}
	return status
}

func getContainerLogs(ctx context.Context, cluster *cluster.Cluster, pod *corev1.Pod, containerName string) ([]byte, error) {
	if containerName == initContainerType {
		return getInitContainerLogs(ctx, cluster, pod)
	}
	if containerName == buildContainerType {
		return getMainContainerLogs(ctx, cluster, pod)
	}
	return nil, fmt.Errorf("container type %s not found", containerName)
}

func getInitContainerLogs(ctx context.Context, cluster *cluster.Cluster, pod *corev1.Pod) ([]byte, error) {
	logs, err := rcommon.GetPodLog(ctx, cluster, pod.Name, pod.Namespace, "init-repo")
	if err != nil {
		return nil, err
	}

	return logs, nil
}

func getMainContainerLogs(ctx context.Context, cluster *cluster.Cluster, pod *corev1.Pod) ([]byte, error) {
	logs, err := rcommon.GetPodLog(ctx, cluster, pod.Name, pod.Namespace, "main")
	if err != nil {
		return nil, err
	}

	return logs, nil
}

func (ibc *imagebuilderComponentImpl) getInitContainerLogsStream(ctx context.Context, cluster *cluster.Cluster, ibw *database.ImageBuilderWork) chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		var err error
		for {
			if ibw.WorkStatus == string(v1alpha1.WorkflowUnknown) {
				ch <- fmt.Sprintf("imagebuilder workflow %s is not running status: %s container init repo status: %s", ibw.WorkName, ibw.WorkStatus, ibw.InitContainerStatus)
				return
			}

			if ibw.InitContainerStatus == containerStatusTerminated {
				ch <- ibw.InitContainerLog
				return
			}

			if ibw.InitContainerStatus == containerStatusRunning {
				logs, msg, err := rcommon.GetPodLogStream(ctx, cluster, ibw.PodName, ibw.Namespace, "init-repo")
				if err != nil {
					ch <- fmt.Sprintf("failed to get container init repo logs: %s", err.Error())
					return
				}
				if msg != "" {
					ch <- msg
				}
				for log := range logs {
					ch <- string(log)
				}
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				ibw, err = ibc.db.QueryStatusByBuildID(ctx, ibw.BuildId)
				if err != nil {
					ch <- fmt.Sprintf("failed to get container init repo status: %s", err.Error())
					return
				}
			}
		}
	}()
	return ch
}

func (ibc *imagebuilderComponentImpl) getMainContainerLogsStream(ctx context.Context, cluster *cluster.Cluster, ibw *database.ImageBuilderWork) chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		var err error
		for {
			if ibw.WorkStatus == string(v1alpha1.WorkflowUnknown) {
				ch <- fmt.Sprintf("imagebuilder workflow %s is not running status: %s", ibw.WorkName, ibw.WorkStatus)
				return
			}
			if ibw.WorkStatus != string(v1alpha1.WorkflowRunning) && ibw.WorkStatus != string(v1alpha1.WorkflowPending) {
				ch <- ibw.MainContainerLog
				return
			}

			if ibw.InitContainerStatus == containerStatusTerminated && ibw.WorkStatus == string(v1alpha1.WorkflowRunning) {
				// get logs from cluster
				logs, msg, err := rcommon.GetPodLogStream(ctx, cluster, ibw.PodName, ibw.Namespace, "main")
				if err != nil {
					ch <- fmt.Sprintf("failed to get container build logs: %s", err.Error())
					return
				}
				if msg != "" {
					ch <- msg
				}
				for log := range logs {
					ch <- string(log)
				}
				return
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				ibw, err = ibc.db.QueryStatusByBuildID(ctx, ibw.BuildId)
				if err != nil {
					ch <- fmt.Sprintf("failed to get container build status: %s", err.Error())
					return
				}
			}
		}
	}()
	return ch
}
