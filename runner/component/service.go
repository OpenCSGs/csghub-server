package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"knative.dev/serving/pkg/client/informers/externalversions"
	listerv1 "knative.dev/serving/pkg/client/listers/serving/v1"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/event"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	rcommon "opencsg.com/csghub-server/runner/common"
	rTypes "opencsg.com/csghub-server/runner/types"
)

var (
	KeyDeployID      string = "deploy_id"
	KeyDeployType    string = "deploy_type"
	KeyUserID        string = "user_id"
	KeyDeploySKU     string = "deploy_sku"
	KeyOrderDetailID string = "order-detail-id"
	KeyMinScale      string = "autoscaling.knative.dev/min-scale"
	KeyMaxScale      string = "autoscaling.knative.dev/max-scale"
	KeyServiceLabel  string = "serving.knative.dev/service"
	KeyRunModeLabel  string = "run-mode"
	ValueMultiHost   string = "multi-host"
)

type serviceComponentImpl struct {
	k8sNameSpace            string
	env                     *config.Config
	spaceDockerRegBase      string
	modelDockerRegBase      string
	imagePullSecret         string
	informerSyncPeriodInMin int
	serviceStore            database.KnativeServiceStore
	clusterPool             *cluster.ClusterPool
	eventPub                *event.EventPublisher
	deployLogStore          database.DeployLogStore
}

type ServiceComponent interface {
	RunService(ctx context.Context, req types.SVCRequest) error
	StopService(ctx context.Context, req types.StopRequest) (*types.StopResponse, error)
	PurgeService(ctx context.Context, req types.PurgeRequest) (*types.PurgeResponse, error)
	UpdateService(ctx context.Context, req types.ModelUpdateRequest) (*types.ModelUpdateResponse, error)
	GenerateService(ctx context.Context, cluster cluster.Cluster, request types.SVCRequest) (*v1.Service, error)
	// get secret from k8s
	// notes: admin should create nim secret "ngc-secret" and "nvidia-nim-secrets" in related namespace before deploy
	GetNimSecret(ctx context.Context, cluster cluster.Cluster) (string, error)
	GetServicePodsWithStatus(ctx context.Context, cluster cluster.Cluster, svcName string, namespace string) (*types.InstanceInfo, error)
	// NewPersistentVolumeClaim creates a new k8s PVC with some default values set.
	NewPersistentVolumeClaim(name string, ctx context.Context, cluster cluster.Cluster, hardware types.HardWare) error
	RunInformer()
	GetServicePods(ctx context.Context, cluster cluster.Cluster, svcName string, namespace string, limit int64) ([]string, error)
	GetAllServiceStatus(ctx context.Context) (map[string]*types.StatusResponse, error)
	GetServiceByName(ctx context.Context, svcName, clusterId string) (*types.StatusResponse, error)
	RemoveServiceForcely(ctx context.Context, cluster *cluster.Cluster, svcName string) error
	GetServiceInfo(ctx context.Context, req types.ServiceRequest) (*types.ServiceInfoResponse, error)
	AddServiceInDB(svc v1.Service, clusterID string) error
	DeleteServiceInDB(svc v1.Service, clusterID string) error
	UpdateServiceInDB(svc v1.Service, clusterID string) error
	PodExist(ctx context.Context, cluster cluster.Cluster, podName string) (bool, error)
	GetPodLogsFromDB(ctx context.Context, cluster cluster.Cluster, podName, svcName string) (string, error)
}

func NewServiceComponent(config *config.Config, clusterPool *cluster.ClusterPool) ServiceComponent {
	domainParts := strings.SplitN(config.Space.InternalRootDomain, ".", 2)
	sc := &serviceComponentImpl{
		k8sNameSpace:            domainParts[0],
		env:                     config,
		spaceDockerRegBase:      config.Space.DockerRegBase,
		modelDockerRegBase:      config.Model.DockerRegBase,
		imagePullSecret:         config.Space.ImagePullSecret,
		informerSyncPeriodInMin: config.Space.InformerSyncPeriodInMin,
		serviceStore:            database.NewKnativeServiceStore(),
		clusterPool:             clusterPool,
		eventPub:                &event.DefaultEventPublisher,
		deployLogStore:          database.NewDeployLogStore(),
	}
	go sc.RunInformer()
	return sc
}

func (s *serviceComponentImpl) GenerateService(ctx context.Context, cluster cluster.Cluster, request types.SVCRequest) (*v1.Service, error) {
	annotations := request.Annotation

	environments := []corev1.EnvVar{}
	appPort := 0
	hardware := request.Hardware
	resReq, nodeSelector := GenerateResources(hardware)
	var err error
	if request.Env != nil {
		// generate env
		for key, value := range request.Env {
			environments = append(environments, corev1.EnvVar{Name: key, Value: value})
		}

		// get app expose port from env with key=port
		val, ok := request.Env["port"]
		if !ok {
			return nil, fmt.Errorf("failed to find port from env")
		}

		appPort, err = strconv.Atoi(val)
		if err != nil {
			return nil, fmt.Errorf("port is not valid number, error: %w", err)
		}
	}

	// fix no gpu request case
	if hardware.Gpu.ResourceName == "" || hardware.Gpu.Num == "" {
		environments = append(environments, corev1.EnvVar{Name: "NVIDIA_VISIBLE_DEVICES", Value: "none"})
	}
	if hardware.Npu.ResourceName == "" || hardware.Npu.Num == "" {
		environments = append(environments, corev1.EnvVar{Name: "ASCEND_VISIBLE_DEVICES", Value: "none"})
	}

	if hardware.Dcu.ResourceName == "" || hardware.Dcu.Num == "" {
		environments = append(environments, corev1.EnvVar{Name: "ENFLAME_VISIBLE_DEVICES", Value: "none"})
	}

	if hardware.Gcu.ResourceName == "" || hardware.Gcu.Num == "" {
		environments = append(environments, corev1.EnvVar{Name: "ROCR_VISIBLE_DEVICES", Value: "none"})
		environments = append(environments, corev1.EnvVar{Name: "TOPS_VISIBLE_DEVICES", Value: "none"})
	}

	if appPort == 0 {
		return nil, fmt.Errorf("app export port is not defined")
	}

	// knative service spec container port
	exposePorts := []corev1.ContainerPort{{
		ContainerPort: int32(appPort),
	}}
	// knative service spec resource requirement
	resources := corev1.ResourceRequirements{
		Limits:   resReq,
		Requests: resReq,
	}

	annotations[KeyDeployID] = strconv.FormatInt(request.DeployID, 10)
	annotations[KeyDeployType] = strconv.Itoa(request.DeployType)
	annotations[KeyUserID] = request.UserID
	annotations[KeyDeploySKU] = request.Sku
	annotations[KeyOrderDetailID] = strconv.FormatInt(request.OrderDetailID, 10)

	containerImg := request.ImageID
	if request.RepoType == string(types.ModelRepo) {
		// choose registry
		// add prefix if image is not full path
		if strings.Count(containerImg, "/") == 1 {
			containerImg = path.Join(s.modelDockerRegBase, request.ImageID)
		}
	} else if request.RepoType == string(types.SpaceRepo) {
		// choose registry
		containerImg = path.Join(s.spaceDockerRegBase, request.ImageID)
	}

	templateAnnotations := make(map[string]string)
	if request.RepoType == string(types.ModelRepo) {
		// auto scaling
		templateAnnotations["autoscaling.knative.dev/class"] = "kpa.autoscaling.knative.dev"
		templateAnnotations["enable-scale-to-zero"] = "false"
		templateAnnotations["autoscaling.knative.dev/metric"] = "concurrency"
		templateAnnotations["autoscaling.knative.dev/target"] = "5"
		templateAnnotations["autoscaling.knative.dev/target-utilization-percentage"] = "90"
		templateAnnotations["autoscaling.knative.dev/min-scale"] = strconv.Itoa(request.MinReplica)
		templateAnnotations["autoscaling.knative.dev/max-scale"] = strconv.Itoa(request.MaxReplica)
		templateAnnotations["serving.knative.dev/progress-deadline"] = fmt.Sprintf("%dm", s.env.Model.DeployTimeoutInMin)
	}
	initialDelaySeconds := 10
	periodSeconds := 10
	failureThreshold := 3
	if request.DeployType == types.InferenceType {
		initialDelaySeconds = s.env.Space.ReadinessDelaySeconds
		periodSeconds = s.env.Space.ReadinessPeriodSeconds
		failureThreshold = s.env.Space.ReadinessFailureThreshold
	}

	imagePullSecrets := []corev1.LocalObjectReference{
		{
			Name: s.imagePullSecret,
		},
	}

	// handle nim engine
	if strings.Contains(containerImg, "nvcr.io/nim/") {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{
			Name: s.env.Model.NimDockerSecretName,
		})
		ngc_api_key, err := s.GetNimSecret(ctx, cluster)
		if err != nil {
			return nil, fmt.Errorf("can not find secret %s in %s namespace , error: %w", s.env.Model.NimNGCSecretName, s.k8sNameSpace, err)
		}
		environments = append(environments, corev1.EnvVar{Name: "NGC_API_KEY", Value: ngc_api_key})
		environments = append(environments, corev1.EnvVar{Name: "NIM_CACHE_PATH", Value: "/workspace"})
	}

	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        request.SvcName,
			Namespace:   s.k8sNameSpace,
			Annotations: annotations,
		},
		Spec: v1.ServiceSpec{
			ConfigurationSpec: v1.ConfigurationSpec{
				Template: v1.RevisionTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: templateAnnotations,
					},
					Spec: v1.RevisionSpec{
						PodSpec: corev1.PodSpec{
							NodeSelector: nodeSelector,
							Containers: []corev1.Container{{
								Image:     containerImg,
								Ports:     exposePorts,
								Resources: resources,
								Env:       environments,
								ReadinessProbe: &corev1.Probe{
									InitialDelaySeconds: int32(initialDelaySeconds),
									PeriodSeconds:       int32(periodSeconds),
									FailureThreshold:    int32(failureThreshold),
								},
							}},
							ImagePullSecrets: imagePullSecrets,
						},
					},
				},
			},
		},
	}
	return service, nil
}

// get secret from k8s
// notes: admin should create nim secret "ngc-secret" and "nvidia-nim-secrets" in related namespace before deploy
func (s *serviceComponentImpl) GetNimSecret(ctx context.Context, cluster cluster.Cluster) (string, error) {
	secret, err := cluster.Client.CoreV1().Secrets(s.k8sNameSpace).Get(ctx, s.env.Model.NimNGCSecretName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return string(secret.Data["NGC_API_KEY"]), nil
}

func (s *serviceComponentImpl) GetServicePodsWithStatus(ctx context.Context, cluster cluster.Cluster, svcName string, namespace string) (*types.InstanceInfo, error) {
	labelSelector := fmt.Sprintf("serving.knative.dev/service=%s", svcName)
	// Get the list of Pods based on the label selector
	pods, err := cluster.Client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	// Extract the Pod names and status
	var podInstances []types.Instance
	var instanceInfo types.InstanceInfo
	readyCount := 0
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp != nil {
			continue
		}
		status := pod.Status.Phase
		// Check container statuses for failure reasons
		_, isPodFailed := hasFailedStatus(&pod)
		if isPodFailed {
			status = corev1.PodFailed
		}
		podInstances = append(podInstances,
			types.Instance{
				Name:   pod.Name,
				Status: string(status),
			},
		)
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				readyCount++
			}
		}
	}
	instanceInfo.Instances = podInstances
	instanceInfo.ReadyCount = readyCount
	message, reason := getPodError(pods)
	if message != nil {
		instanceInfo.Message = *message
		instanceInfo.Reason = *reason
	}
	return &instanceInfo, nil
}

func hasFailedStatus(pod *corev1.Pod) (string, bool) {
	if pod == nil || len(pod.Status.ContainerStatuses) == 0 {
		return "", false
	}

	failureReasons := map[string]struct{}{
		"CrashLoopBackOff": {},
		"ErrImagePull":     {},
		"ImagePullBackOff": {},
		"OOMKilled":        {},
	}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name != "user-container" {
			continue
		}
		if cs.State.Waiting != nil {
			if _, exists := failureReasons[cs.State.Waiting.Reason]; exists {
				return cs.State.Waiting.Reason, true
			}
		}
		if cs.State.Terminated != nil {
			if _, exists := failureReasons[cs.State.Terminated.Reason]; exists {
				return cs.State.Terminated.Reason, true
			}
		}
	}
	return "", false
}

func isProxyReady(pod *corev1.Pod) bool {
	if pod == nil || len(pod.Status.ContainerStatuses) == 0 {
		return false
	}

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name != "queue-proxy" {
			continue
		}
		return cs.Ready
	}
	return false
}

// pod is in ContainerCreating
func isContainerCreating(pod *corev1.Pod) bool {
	if pod == nil || len(pod.Status.ContainerStatuses) == 0 {
		return false
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name != "user-container" {
			continue
		}
		if cs.State.Waiting != nil && cs.State.Waiting.Reason == "ContainerCreating" {
			return true
		}
	}
	return false
}

func getPodError(podList *corev1.PodList) (*string, *string) {
	for _, pod := range podList.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name != "user-container" {
				continue
			}
			if cs.LastTerminationState.Terminated != nil {
				lastState := cs.LastTerminationState.Terminated
				return &lastState.Message, &lastState.Reason
			}
		}
	}
	return nil, nil
}

func GenerateResources(hardware types.HardWare) (map[corev1.ResourceName]resource.Quantity, map[string]string) {
	nodeSelector := make(map[string]string)
	resReq := make(map[corev1.ResourceName]resource.Quantity)

	// Helper function to process labels
	addLabels := func(labels map[string]string) {
		for key, value := range labels {
			nodeSelector[key] = value
		}
	}

	// Process all hardware labels
	hardwareTypes := []struct {
		labels map[string]string
	}{
		{hardware.Gpu.Labels},
		{hardware.Npu.Labels},
		{hardware.Gcu.Labels},
		{hardware.Mlu.Labels},
		{hardware.Dcu.Labels},
		{hardware.GPGpu.Labels},
		{hardware.Cpu.Labels},
	}

	for _, hw := range hardwareTypes {
		if hw.labels != nil {
			addLabels(hw.labels)
		}
	}

	// Helper function to parse resource quantities
	parseResource := func(value string) resource.Quantity {
		if value == "" {
			return resource.Quantity{}
		}
		return resource.MustParse(value)
	}

	// Process CPU resources
	if hardware.Cpu.Num != "" {
		qty := parseResource(hardware.Cpu.Num)
		resReq[corev1.ResourceCPU] = qty
	}

	// Process memory resources
	if hardware.Memory != "" {
		qty := parseResource(hardware.Memory)
		resReq[corev1.ResourceMemory] = qty
	}

	// Process ephemeral storage
	if hardware.EphemeralStorage != "" {
		qty := parseResource(hardware.EphemeralStorage)
		resReq[corev1.ResourceEphemeralStorage] = qty
	}

	// Process accelerator resources
	accelerators := []struct {
		resourceName string
		num          string
	}{
		{hardware.Gpu.ResourceName, hardware.Gpu.Num},
		{hardware.Npu.ResourceName, hardware.Npu.Num},
		{hardware.Gcu.ResourceName, hardware.Gcu.Num},
		{hardware.Mlu.ResourceName, hardware.Mlu.Num},
		{hardware.Dcu.ResourceName, hardware.Dcu.Num},
		{hardware.GPGpu.ResourceName, hardware.GPGpu.Num},
	}

	for _, acc := range accelerators {
		if acc.resourceName != "" && acc.num != "" {
			qty := parseResource(acc.num)
			resReq[corev1.ResourceName(acc.resourceName)] = qty
		}
	}

	return resReq, nodeSelector
}

// NewPersistentVolumeClaim creates a new k8s PVC with some default values set.
func (s *serviceComponentImpl) NewPersistentVolumeClaim(name string, ctx context.Context, cluster cluster.Cluster, hardware types.HardWare) error {
	// Check if it already exists
	_, err := cluster.Client.CoreV1().PersistentVolumeClaims(s.k8sNameSpace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	storageSize := hardware.EphemeralStorage
	if storageSize == "" {
		storageSize = "50Gi"
	}

	storage, err := resource.ParseQuantity(storageSize)
	if err != nil {
		return err
	}
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.k8sNameSpace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storage,
				},
			},
			StorageClassName: &cluster.StorageClass,
		},
	}
	_, err = cluster.Client.CoreV1().PersistentVolumeClaims(s.k8sNameSpace).Create(ctx, &pvc, metav1.CreateOptions{})
	return err
}

func (s *serviceComponentImpl) RunInformer() {
	var wg sync.WaitGroup
	stopCh := make(chan struct{})
	defer close(stopCh)
	defer runtime.HandleCrash()
	for _, cls := range s.clusterPool.Clusters {
		_, err := cls.Client.Discovery().ServerVersion()
		if err != nil {
			slog.Error("cluster is unavailable ", slog.Any("cluster config", cls.CID), slog.Any("error", err))
			continue
		}
		wg.Add(2)
		go func(cluster cluster.Cluster) {
			defer wg.Done()
			s.RunServiceInformer(stopCh, cluster)
		}(cls)
		go func(cluster cluster.Cluster) {
			defer wg.Done()
			s.RunPodInformer(stopCh, cluster)
		}(cls)
	}
	wg.Wait()
}

// Run service informer, main handle the service changes
func (s *serviceComponentImpl) RunServiceInformer(stopCh <-chan struct{}, cluster cluster.Cluster) {
	informerFactory := externalversions.NewSharedInformerFactoryWithOptions(
		cluster.KnativeClient,
		time.Duration(s.informerSyncPeriodInMin)*time.Minute, //sync every 2 minutes, if network unavailable, it will trigger watcher to reconnect
		externalversions.WithNamespace(s.k8sNameSpace),
	)
	informer := informerFactory.Serving().V1().Services().Informer()
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			service := obj.(*v1.Service)
			slog.Debug("add knative server by informer", slog.Any("clusterID", cluster.ID), slog.Any("service", service.Name))
			err := s.AddServiceInDB(*service, cluster.ID)
			if err != nil {
				slog.Error("failed to add service ", slog.Any("service", service.Name), slog.Any("error", err))
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			new := newObj.(*v1.Service)
			old := oldObj.(*v1.Service)
			newStatus := getReadyCondition(new)
			oldStatus := getReadyCondition(old)
			if newStatus != oldStatus {
				err := s.UpdateServiceInDB(*new, cluster.ID)
				if err != nil {
					slog.Error("failed to update service status ", slog.Any("service", new.Name), slog.Any("error", err))
				}
			}
		},
		DeleteFunc: func(obj any) {
			service := obj.(*v1.Service)
			slog.Debug("delete knative server by informer", slog.Any("clusterID", cluster.ID), slog.Any("service", service.Name))
			err := s.DeleteServiceInDB(*service, cluster.ID)
			if err != nil {
				slog.Error("failed to mark service as deleted ", slog.Any("service", service.Name), slog.Any("error", err))
			}
		},
	})
	if err != nil {
		runtime.HandleError(fmt.Errorf("failed to add event handler for knative service informer"))
	}
	go s.RunServiceSync(informerFactory, cluster)
	informer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync for knative service informer"))
	}

}

// run svc sync
func (s *serviceComponentImpl) RunServiceSync(informerFactory externalversions.SharedInformerFactory, cluster cluster.Cluster) {
	lister := informerFactory.Serving().V1().Services().Lister()
	//sync once at start
	s.syncServiceInDB(lister, cluster)
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		s.syncServiceInDB(lister, cluster)
	}
}

// compare and remove service list in db
func (s *serviceComponentImpl) syncServiceInDB(lister listerv1.ServiceLister, cluster cluster.Cluster) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		services []*v1.Service
		err      error
	)

	services, err = lister.Services(s.k8sNameSpace).List(labels.Everything())

	if err != nil {
		slog.Error("failed to list services", slog.Any("error", err))
		return
	}

	if len(services) < 1 {
		slog.Debug("skip sync service for start runner first time")
		return
	}

	localServices, err := s.serviceStore.GetByCluster(ctx, cluster.ID)
	if err != nil {
		slog.Error("failed to list services in db", slog.Any("error", err))
		return
	}
	slog.Debug("run sync service to compare with db", slog.Any("local services", len(localServices)), slog.Any("cluster", cluster.ID), slog.Any("services", len(services)))
	for _, localService := range localServices {
		found := false
		for _, service := range services {
			if localService.Name == service.Name {
				found = true
				break
			}
		}
		if !found {
			slog.Debug("delete service in sync", slog.Any("service", localService.Name), slog.Any("cluster", cluster.ID))
			err := s.serviceStore.Delete(ctx, cluster.ID, localService.Name)
			if err != nil {
				slog.Error("failed to delete service ", slog.Any("service", localService.Name), slog.Any("error", err))
			}
		}
	}
}

func (s *serviceComponentImpl) RunPodInformer(stopCh <-chan struct{}, cluster cluster.Cluster) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		cluster.Client,
		1*time.Hour, //sync every 1 hour, if pod unavailable, it will reconnect
		informers.WithNamespace(s.k8sNameSpace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = KeyServiceLabel
		}),
	)

	// Get pod informer
	podInformer := factory.Core().V1().Pods()

	// Add event handler
	_, err := podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj any) {
			new := newObj.(*corev1.Pod)
			serviceName := new.Labels[KeyServiceLabel]
			podStatus, isPodFailed := hasFailedStatus(new)
			if serviceName != "" && (isPodFailed || isProxyReady(new) || isContainerCreating(new)) {
				slog.Debug("pod status changed by informer", slog.Any("service", serviceName),
					slog.Any("pod-name", new.Name), slog.Any("namespace", new.Namespace),
					slog.Any("isPodFailed", isPodFailed), slog.Any("pod-status", podStatus))
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				svc, err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).
					Get(ctx, serviceName, metav1.GetOptions{})
				if err != nil {
					slog.Error("failed to get service", slog.Any("service", serviceName), slog.Any("error", err))
				}
				err = s.UpdateServiceInDB(*svc, cluster.ID)
				if err != nil {
					slog.Error("failed to update service status", slog.Any("service", serviceName), slog.Any("error", err))
				}
				if isPodFailed && svc != nil && len(svc.Name) > 0 {
					err = s.persistPodLogs(*svc, new, podStatus, &cluster)
					if err != nil {
						slog.Error("failed to persist pod logs", slog.Any("service", serviceName), slog.Any("error", err))
					}
				}
			}
		},
		DeleteFunc: func(obj any) {
			switch new := obj.(type) {
			case *corev1.Pod:
				serviceName := new.Labels[KeyServiceLabel]
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				srv, err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).
					Get(ctx, serviceName, metav1.GetOptions{})
				if err != nil {
					slog.Error("failed to get service ", slog.Any("service", serviceName), slog.Any("error", err))
				}
				err = s.UpdateServiceInDB(*srv, cluster.ID)
				if err != nil {
					slog.Error("failed to update service status ", slog.Any("service", serviceName), slog.Any("error", err))
				}
			default:
				return
			}
		},
	})

	if err != nil {
		runtime.HandleError(fmt.Errorf("failed to add event handler for pod informer"))
	}

	// Start informer
	factory.Start(stopCh)

	// Wait for cache sync
	if !cache.WaitForCacheSync(stopCh, podInformer.Informer().HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
	}
	<-stopCh
}

func (s *serviceComponentImpl) AddServiceInDB(svc v1.Service, clusterID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	status, err := s.GetServiceStatus(ctx, svc, clusterID)
	if err != nil {
		return err
	}
	deployIDStr := svc.Annotations[KeyDeployID]
	deployID, _ := strconv.ParseInt(deployIDStr, 10, 64)
	deployTypeStr := svc.Annotations[KeyDeployType]
	deployType, err := strconv.Atoi(deployTypeStr)
	if err != nil {
		deployType = 0
	}
	userID := svc.Annotations[KeyUserID]
	deploySku := svc.Annotations[KeyDeploySKU]
	orderDetailIdStr := svc.Annotations[KeyOrderDetailID]
	orderDetailId, err := strconv.ParseInt(orderDetailIdStr, 10, 64)
	if err != nil {
		orderDetailId = 0
	}
	deployment, err := s.getDeploymentByServiceName(ctx, svc, clusterID)
	if err != nil {
		slog.Error("failed to get deployment", slog.Any("service", svc.Name), slog.Any("error", err))
	}
	service := &database.KnativeService{
		Code:          status.Code,
		Name:          svc.Name,
		ClusterID:     clusterID,
		Status:        getReadyCondition(&svc),
		Endpoint:      svc.Status.URL.String(),
		DeployID:      deployID,
		UserUUID:      userID,
		DeployType:    deployType,
		DeploySKU:     deploySku,
		OrderDetailID: orderDetailId,
		Instances:     status.Instances,
	}
	if deployment != nil {
		desiredReplicas := 1
		if deployment.Spec.Replicas != nil {
			desiredReplicas = int(*deployment.Spec.Replicas)
		}
		service.DesiredReplica = desiredReplicas
		service.ActualReplica = int(deployment.Status.Replicas)
	}

	return s.serviceStore.Add(ctx, service)
}

// Delete service, just mark the service as stopped
func (s *serviceComponentImpl) DeleteServiceInDB(svc v1.Service, clusterID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	event := types.ServiceEvent{
		ServiceName: svc.Name,
		Status:      common.Stopped,
	}
	s.sendServiceUpdateEvent(event)

	return s.serviceStore.Delete(ctx, clusterID, svc.Name)
}

func (s *serviceComponentImpl) UpdateServiceInDB(svc v1.Service, clusterID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	status, err := s.GetServiceStatus(ctx, svc, clusterID)
	if err != nil {
		return err
	}
	oldService, err := s.serviceStore.Get(ctx, svc.Name, clusterID)
	if err != nil {
		return err
	}
	deployment, err := s.getDeploymentByServiceName(ctx, svc, clusterID)
	if err != nil {
		slog.Error("failed to get deployment ", slog.Any("service", svc.Name), slog.Any("error", err))
	}
	oldService.Endpoint = svc.Status.URL.String()
	oldService.Status = getReadyCondition(&svc)
	oldService.Instances = status.Instances
	if deployment != nil {
		desiredReplicas := 1
		if deployment.Spec.Replicas != nil {
			desiredReplicas = int(*deployment.Spec.Replicas)
		}
		oldService.DesiredReplica = desiredReplicas
		oldService.ActualReplica = int(deployment.Status.Replicas)
	}
	if oldService.Code != status.Code || status.Message != "" {
		event := types.ServiceEvent{
			ServiceName: svc.Name,
			Status:      status.Code,
			Message:     status.Message,
			Reason:      status.Reason,
		}
		s.sendServiceUpdateEvent(event)
	}
	oldService.Code = status.Code
	return s.serviceStore.Update(ctx, oldService)
}

// Get Revision by Service name
func (s *serviceComponentImpl) getDeploymentByServiceName(ctx context.Context, service v1.Service, clusterID string) (*appsv1.Deployment, error) {

	// Extract the latest ready Revision name
	svcName := service.Name
	cluster, err := s.clusterPool.GetClusterByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("fail to get cluster, error: %w", err)
	}
	labelSelector := fmt.Sprintf("serving.knative.dev/service=%s", svcName)
	deployments, err := cluster.Client.AppsV1().Deployments(s.k8sNameSpace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil || len(deployments.Items) == 0 {
		return nil, fmt.Errorf("fail to get deployment list by selector %s, error: %w", labelSelector, err)
	}
	return &deployments.Items[0], nil
}

func (s *serviceComponentImpl) GetServiceStatus(ctx context.Context, ks v1.Service, clusterID string) (resp types.StatusResponse, err error) {
	serviceCondition := ks.Status.GetCondition(v1.ServiceConditionReady)
	cluster, err := s.clusterPool.GetClusterByID(ctx, clusterID)
	if err != nil {
		return resp, fmt.Errorf("fail to get cluster,error: %v ", err)
	}
	instInfo, err := s.GetServicePodsWithStatus(ctx, *cluster, ks.Name, ks.Namespace)
	if err != nil {
		return resp, fmt.Errorf("fail to get service pod name list,error: %v ", err)
	}

	switch {
	case serviceCondition == nil:
		resp.Code = common.Deploying
	case serviceCondition.Status == corev1.ConditionUnknown:
		resp.Code = common.DeployFailed
		if isUserContainerActive(instInfo.Instances) {
			resp.Code = common.Deploying
		}
	case serviceCondition.Status == corev1.ConditionTrue:
		resp.Code = common.Running
		if len(instInfo.Instances) == 0 {
			resp.Code = common.Sleeping
		} else if instInfo.ReadyCount < 2 {
			//for wakeup case
			resp.Code = common.Deploying
		}
	case serviceCondition.Status == corev1.ConditionFalse:
		resp.Code = common.DeployFailed
		if isUserContainerActive(instInfo.Instances) {
			resp.Code = common.Deploying
		}
	}
	resp.Message = instInfo.Message
	resp.Instances = instInfo.Instances
	resp.Reason = instInfo.Reason
	return resp, err
}
func isUserContainerActive(instList []types.Instance) bool {
	for _, instance := range instList {
		if instance.Status == string(corev1.PodRunning) || instance.Status == string(corev1.PodPending) {
			return true
		}
	}
	return false
}

// corev1.ConditionTrue
func getReadyCondition(service *v1.Service) corev1.ConditionStatus {
	for _, condition := range service.Status.Conditions {
		if condition.Type == v1.ServiceConditionReady {
			return condition.Status
		}
	}
	return corev1.ConditionUnknown
}

func (s *serviceComponentImpl) GetServicePods(ctx context.Context, cluster cluster.Cluster, svcName string, namespace string, limit int64) ([]string, error) {
	labelSelector := fmt.Sprintf("serving.knative.dev/service=%s", svcName)
	// Get the list of Pods based on the label selector
	opts := metav1.ListOptions{
		LabelSelector: labelSelector,
	}
	if limit > 0 {
		opts = metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         limit,
		}
	}
	pods, err := cluster.Client.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}

	// Extract the Pod names
	var podNames []string
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp != nil {
			continue
		}
		podNames = append(podNames, pod.Name)
	}

	return podNames, nil
}

// GetAllServiceStatus
func (s *serviceComponentImpl) GetAllServiceStatus(ctx context.Context) (map[string]*types.StatusResponse, error) {
	allStatus := make(map[string]*types.StatusResponse)
	for _, cls := range s.clusterPool.Clusters {
		svcs, err := s.serviceStore.GetByCluster(ctx, cls.ID)
		if err != nil {
			return nil, fmt.Errorf("fail to get service list,error: %v ", err)
		}
		for _, svc := range svcs {
			status := &types.StatusResponse{
				DeployID:      svc.DeployID,
				UserID:        svc.UserUUID,
				DeployType:    svc.DeployType,
				ServiceName:   svc.Name,
				DeploySku:     svc.DeploySKU,
				OrderDetailID: svc.OrderDetailID,
				Code:          svc.Code,
			}
			allStatus[svc.Name] = status
		}
	}
	return allStatus, nil
}

// GetServiceStatus
func (s *serviceComponentImpl) GetServiceByName(ctx context.Context, svcName, clusterId string) (*types.StatusResponse, error) {
	if clusterId == "" {
		// use default value
		cluster, err := s.clusterPool.GetClusterByID(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("fail to get cluster ,error: %v ", err)
		}
		clusterId = cluster.ID
	}
	svc, err := s.serviceStore.Get(ctx, svcName, clusterId)
	if err != nil {
		return nil, err
	}
	resp := &types.StatusResponse{
		DeployID:      svc.DeployID,
		UserID:        svc.UserUUID,
		DeployType:    svc.DeployType,
		ServiceName:   svc.Name,
		DeploySku:     svc.DeploySKU,
		OrderDetailID: svc.OrderDetailID,
		Endpoint:      svc.Endpoint,
		Code:          svc.Code,
		Instances:     svc.Instances,
		Replica:       len(svc.Instances),
	}
	return resp, nil

}

func (s *serviceComponentImpl) RunService(ctx context.Context, req types.SVCRequest) error {
	if req.Hardware.Replicas > 1 {
		return s.RunServiceMultiHost(ctx, req)
	} else {
		return s.RunServiceSingleHost(ctx, req)
	}
}

// RunService
func (s *serviceComponentImpl) RunServiceSingleHost(ctx context.Context, req types.SVCRequest) error {
	cluster, err := s.clusterPool.GetClusterByID(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("fail to get cluster, error %v ", err)
	}

	// check if the ksvc exists
	_, err = cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Get(ctx, req.SvcName, metav1.GetOptions{})
	if err == nil {
		err = s.RemoveServiceForcely(ctx, cluster, req.SvcName)
		if err != nil {
			slog.Error("fail to remove service", slog.Any("error", err), slog.Any("req", req))
		}
		slog.Info("service already exists,delete it first", slog.String("svc_name", req.SvcName), slog.Any("image_id", req.ImageID))
	}
	service, err := s.GenerateService(ctx, *cluster, req)
	if err != nil {
		return fmt.Errorf("fail to generate service, %v ", err)
	}
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	if req.DeployType != types.SpaceType {
		// dshm volume for multi-gpu share memory
		volumes = append(volumes, corev1.Volume{
			Name: "dshm",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "dshm",
			MountPath: "/dev/shm",
		})
	}
	pvcName := req.SvcName
	if req.DeployType == types.InferenceType {
		pvcName = req.UserID
	}
	// add pvc if possible
	// space image was built from user's code, model cache dir is hard to control
	// so no PV cache for space case so far
	if cluster.StorageClass != "" && req.DeployType != types.SpaceType {
		err = s.NewPersistentVolumeClaim(pvcName, ctx, *cluster, req.Hardware)
		if err != nil {
			return fmt.Errorf("failed to create persist volume, %v", err)
		}
		volumes = append(volumes, corev1.Volume{
			Name: "nas-pvc",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "nas-pvc",
			MountPath: "/workspace",
		})
	}
	service.Spec.Template.Spec.Volumes = volumes
	service.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	slog.Debug("creating ksvc", slog.Any("knative service", service))

	// create ksvc
	_, err = cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service, error: %v, req: %v", err, req)
	}
	// add a placeholder service
	req.ClusterID = cluster.ID
	err = s.AddKService(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to add knative service to db, error: %v", err)
	}
	return nil
}

func (s *serviceComponentImpl) RemoveServiceForcely(ctx context.Context, cluster *cluster.Cluster, svcName string) error {
	err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Delete(context.Background(), svcName, *metav1.NewDeleteOptions(0))
	if err != nil {
		return err
	}
	podNames, _ := s.GetServicePods(ctx, *cluster, svcName, s.k8sNameSpace, -1)
	if podNames == nil {
		return nil
	}
	//before k8s 1.31, kill pod does not kill the process immediately, instead we still need wait for the process to exit. more details see: https://github.com/kubernetes/kubernetes/issues/120449
	gracePeriodSeconds := int64(10)
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
		PropagationPolicy:  &deletePolicy,
	}

	for _, podName := range podNames {
		errForce := cluster.Client.CoreV1().Pods(s.k8sNameSpace).Delete(ctx, podName, deleteOptions)
		if errForce != nil {
			slog.Error("removeServiceForcely failed to delete pod", slog.String("pod_name", podName), slog.Any("error", errForce))
		}
	}
	return nil
}

// StopService
func (s *serviceComponentImpl) StopService(ctx context.Context, req types.StopRequest) (*types.StopResponse, error) {
	var resp types.StopResponse
	cluster, err := s.clusterPool.GetClusterByID(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("fail to get cluster, error: %v ", err)
	}

	srv, err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).
		Get(ctx, req.SvcName, metav1.GetOptions{})
	if err != nil {
		k8serr := new(k8serrors.StatusError)
		if errors.As(err, &k8serr) {
			if k8serr.Status().Code == http.StatusNotFound {
				slog.Info("stop service skip,service not exist", slog.String("svc_name", req.SvcName), slog.Any("k8s_err", k8serr))
				resp.Code = 0
				resp.Message = "skip,service not exist"
				return &resp, nil
			}
		}
		resp.Code = -1
		resp.Message = "failed to get service status"
		return &resp, fmt.Errorf("cannot get service info, error: %v", err)
	}

	if srv == nil {
		resp.Code = 0
		resp.Message = "service not exist"
		return &resp, nil
	}
	err = s.RemoveServiceForcely(ctx, cluster, req.SvcName)
	if err != nil {
		resp.Code = -1
		resp.Message = "failed to get service status"
		return &resp, fmt.Errorf("cannot delete service,error: %v", err)
	}
	err = s.RemoveWorkset(ctx, *cluster, srv)
	if err != nil {
		resp.Code = -1
		resp.Message = "failed to remove workset pod"
		return &resp, fmt.Errorf("failed to remove workset pod, error: %v", err)
	}
	err = s.serviceStore.Delete(ctx, cluster.ID, req.SvcName)
	if err != nil {
		resp.Code = -1
		resp.Message = "failed to delete service"
		return &resp, fmt.Errorf("cannot delete service info from db, error: %v", err)
	}
	return &resp, nil
}

// UpdateService
func (s *serviceComponentImpl) UpdateService(ctx context.Context, req types.ModelUpdateRequest) (*types.ModelUpdateResponse, error) {
	var resp types.ModelUpdateResponse
	cluster, err := s.clusterPool.GetClusterByID(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("fail to get cluster, error: %v ", err)
	}

	srv, err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).
		Get(ctx, req.SvcName, metav1.GetOptions{})
	if err != nil {
		k8serr := new(k8serrors.StatusError)
		if errors.As(err, &k8serr) {
			if k8serr.Status().Code == http.StatusNotFound {
				resp.Code = 0
				resp.Message = "skipped, service not exist"
				return &resp, nil
			}
		}
		resp.Code = -1
		resp.Message = "failed to get service status"
		return &resp, fmt.Errorf("cannot get service info, error: %v", err)
	}

	if srv == nil {
		resp.Code = 0
		resp.Message = "service not exist"
		return &resp, nil
	}
	// Update Image
	containerImg := path.Join(s.modelDockerRegBase, req.ImageID)
	srv.Spec.Template.Spec.Containers[0].Image = containerImg
	// Update env
	environments := []corev1.EnvVar{}
	if req.Env != nil {
		// generate env
		for key, value := range req.Env {
			environments = append(environments, corev1.EnvVar{Name: key, Value: value})
		}
		srv.Spec.Template.Spec.Containers[0].Env = environments
	}
	// Update CPU and Memory requests and limits
	hardware := req.Hardware
	resReq, nodeSelector := GenerateResources(hardware)
	resources := corev1.ResourceRequirements{
		Limits:   resReq,
		Requests: resReq,
	}
	srv.Spec.Template.Spec.Containers[0].Resources = resources
	srv.Spec.Template.Spec.NodeSelector = nodeSelector
	// Update replica
	srv.Spec.Template.Annotations["autoscaling.knative.dev/min-scale"] = strconv.Itoa(req.MinReplica)
	srv.Spec.Template.Annotations["autoscaling.knative.dev/max-scale"] = strconv.Itoa(req.MaxReplica)

	_, err = cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Update(ctx, srv, metav1.UpdateOptions{})
	if err != nil {
		resp.Code = -1
		resp.Message = "failed to update service"
		return &resp, fmt.Errorf("cannot update service, error: %v", err)
	}
	return &resp, nil
}

func (s *serviceComponentImpl) PurgeService(ctx context.Context, req types.PurgeRequest) (*types.PurgeResponse, error) {
	var resp types.PurgeResponse
	cluster, err := s.clusterPool.GetClusterByID(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("fail to get cluster, error: %v ", err)
	}
	ksvc, err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).
		Get(ctx, req.SvcName, metav1.GetOptions{})
	if err != nil {
		k8serr := new(k8serrors.StatusError)
		if errors.As(err, &k8serr) {
			if k8serr.Status().Code == http.StatusNotFound {
				slog.Info("service not exist", slog.String("svc_name", req.SvcName), slog.Any("k8s_err", k8serr))
			}
		}
		slog.Error("cannot get service info, skip service purge", slog.String("svc_name", req.SvcName), slog.Any("error", err))
	} else {
		// 1 delete service
		err = s.RemoveServiceForcely(ctx, cluster, req.SvcName)
		if err != nil {
			resp.Code = -1
			resp.Message = "failed to remove service"
			return &resp, fmt.Errorf("failed to remove service, error: %v", err)
		}
	}
	// 2 clean up pvc
	if cluster.StorageClass != "" && req.DeployType == types.FinetuneType {
		err = cluster.Client.CoreV1().PersistentVolumeClaims(s.k8sNameSpace).Delete(ctx, req.SvcName, metav1.DeleteOptions{})
		if err != nil {
			resp.Code = -1
			resp.Message = "failed to remove pvc"
			return &resp, fmt.Errorf("failed to remove pvc, error: %v", err)
		}
	}
	// 3 clean up workset pod
	err = s.RemoveWorkset(ctx, *cluster, ksvc)
	if err != nil {
		resp.Code = -1
		resp.Message = "failed to remove workset pod"
		return &resp, fmt.Errorf("failed to remove workset pod, error: %v", err)
	}

	return &resp, nil
}

// GetServiceInfo
func (s *serviceComponentImpl) GetServiceInfo(ctx context.Context, req types.ServiceRequest) (*types.ServiceInfoResponse, error) {
	var resp types.ServiceInfoResponse
	svc, err := s.serviceStore.Get(ctx, req.ServiceName, req.ClusterID)
	if err != nil {
		return nil, err
	}
	resp.ServiceName = svc.Name
	for _, v := range svc.Instances {
		resp.PodNames = append(resp.PodNames, v.Name)
	}
	return &resp, nil
}

func (wc *serviceComponentImpl) sendServiceUpdateEvent(event types.ServiceEvent) {
	str, err := json.Marshal(event)
	if err != nil {
		slog.Error("error marshal service event", slog.Any("event", event), slog.Any("error", err))
		return
	}
	err = wc.eventPub.PublishServiceEvent(str)
	if err != nil {
		slog.Error("failed to pub service event", slog.Any("data", string(str)), slog.Any("error", err))
	} else {
		slog.Debug("pub service event success", slog.Any("data", string(str)))
	}
}

// add place holder service, the status will be updated by the informer
func (s *serviceComponentImpl) AddKService(ctx context.Context, req types.SVCRequest) error {
	ks := &database.KnativeService{
		Code:       common.Deploying,
		Name:       req.SvcName,
		ClusterID:  req.ClusterID,
		Status:     corev1.ConditionUnknown,
		DeployID:   req.DeployID,
		UserUUID:   req.UserID,
		DeployType: req.DeployType,
	}
	return s.serviceStore.Add(ctx, ks)
}

func (s *serviceComponentImpl) persistPodLogs(svc v1.Service, pod *corev1.Pod, podStatus string, cluster *cluster.Cluster) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	logs, err := rcommon.GetPodLog(ctx, cluster, pod.Name, pod.Namespace, rTypes.UserContainerName)
	if err != nil {
		return fmt.Errorf("failed to get svc %s/%s pod %s/%s log, error: %w", svc.Namespace, svc.Name, pod.Namespace, pod.Name, err)
	}
	logStr := string(logs)
	slog.Debug("persist pod log by status changed", slog.Any("clusterID", cluster.ID), slog.Any("namespace", pod.Namespace),
		slog.Any("svcname", svc.Name), slog.Any("pod-name", pod.Name), slog.Any("log-in-bytes", len(logs)))
	deploy_ids := svc.Annotations[KeyDeployID]
	deploy_id, err := strconv.Atoi(deploy_ids)
	if err != nil {
		slog.Error("failed to convert deploy id", slog.Any("deploy_id", deploy_ids), slog.Any("error", err))
	}
	logData := database.DeployLog{
		DeployID:         int64(deploy_id),
		ClusterID:        cluster.ID,
		SvcName:          svc.Name,
		PodName:          pod.Name,
		PodStatus:        podStatus,
		UserContainerLog: logStr,
	}
	_, err = s.deployLogStore.UpdateDeployLogs(ctx, logData)
	if err != nil {
		return fmt.Errorf("failed to persist svc %s/%s pod %s/%s log, error: %w", svc.Namespace, svc.Name, pod.Namespace, pod.Name, err)
	}
	return nil
}

func (s *serviceComponentImpl) PodExist(ctx context.Context, cluster cluster.Cluster, podName string) (bool, error) {
	_, err := cluster.Client.CoreV1().Pods(s.k8sNameSpace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		k8serr := new(k8serrors.StatusError)
		if errors.As(err, &k8serr) {
			if k8serr.Status().Code == http.StatusNotFound {
				return false, nil
			}
		}
		return false, fmt.Errorf("get pod %s/%s from cluster, error: %w", s.k8sNameSpace, podName, err)
	}
	return true, nil
}

func (s *serviceComponentImpl) GetPodLogsFromDB(ctx context.Context, cluster cluster.Cluster, podName, svcName string) (string, error) {
	logReq := database.DeployLog{
		ClusterID: cluster.ID,
		SvcName:   svcName,
		PodName:   podName,
	}
	deployLog, err := s.deployLogStore.GetDeployLogs(ctx, logReq)
	if err != nil {
		return "", fmt.Errorf("failed to get svc %s/%s pod %s/%s log from db, error: %w", s.k8sNameSpace, svcName, s.k8sNameSpace, podName, err)
	}
	return deployLog.UserContainerLog, nil
}
