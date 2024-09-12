package component

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

var (
	KeyDeployID   string = "deploy_id"
	KeyDeployType string = "deploy_type"
	KeyUserID     string = "user_id"
	KeyDeploySKU  string = "deploy_sku"
)

type ServiceComponent struct {
	k8sNameSpace       string
	env                *config.Config
	spaceDockerRegBase string
	modelDockerRegBase string
	imagePullSecret    string
}

func NewServiceComponent(config *config.Config, k8sNameSpace string) *ServiceComponent {
	sc := &ServiceComponent{
		k8sNameSpace:       k8sNameSpace,
		env:                config,
		spaceDockerRegBase: config.Space.DockerRegBase,
		modelDockerRegBase: config.Model.DockerRegBase,
		imagePullSecret:    config.Space.ImagePullSecret,
	}
	return sc
}

func (s *ServiceComponent) GenerateService(request types.SVCRequest, srvName string) (*v1.Service, error) {
	annotations := request.Annotation

	environments := []corev1.EnvVar{}
	appPort := 0
	hardware := request.Hardware
	resReq, nodeSelector := s.GenerateResources(hardware)
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

	containerImg := path.Join(s.spaceDockerRegBase, request.ImageID)
	if request.RepoType == string(types.ModelRepo) {
		// choose registry
		containerImg = path.Join(s.modelDockerRegBase, request.ImageID)
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
		initialDelaySeconds = s.env.Space.ReadnessDelaySeconds
		periodSeconds = s.env.Space.ReadnessPeriodSeconds
		failureThreshold = s.env.Space.ReadnessFailureThreshold
	}

	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        srvName,
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
								// TODO: docker registry url + image id
								// Image: "ghcr.io/knative/helloworld-go:latest",
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
							ImagePullSecrets: []corev1.LocalObjectReference{
								{
									Name: s.imagePullSecret,
								},
							},
						},
					},
				},
			},
		},
	}
	return service, nil
}

func (s *ServiceComponent) GetServicePodsWithStatus(ctx context.Context, cluster cluster.Cluster, srvName string, namespace string) ([]types.Instance, error) {
	labelSelector := fmt.Sprintf("serving.knative.dev/service=%s", srvName)
	// Get the list of Pods based on the label selector
	pods, err := cluster.Client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	// Extract the Pod names and status
	var podInstances []types.Instance
	for _, pod := range pods.Items {
		podInstances = append(podInstances,
			types.Instance{
				Name:   pod.Name,
				Status: string(pod.Status.Phase),
			},
		)
		slog.Debug("pod", slog.Any("pod.Name", pod.Name), slog.Any("pod.Status.Phase", pod.Status.Phase))
	}
	return podInstances, nil
}

func (s *ServiceComponent) GenerateResources(hardware types.HardWare) (map[corev1.ResourceName]resource.Quantity, map[string]string) {
	nodeSelector := make(map[string]string)
	resReq := make(map[corev1.ResourceName]resource.Quantity)

	// generate node selector
	if hardware.Gpu.Labels != nil {
		for key, value := range hardware.Gpu.Labels {
			nodeSelector[key] = value
		}
	}
	if hardware.Cpu.Labels != nil {
		for key, value := range hardware.Cpu.Labels {
			nodeSelector[key] = value
		}
	}

	// generate knative resource requirement
	if hardware.Cpu.Num != "" {
		resReq[corev1.ResourceCPU] = resource.MustParse(hardware.Cpu.Num)
	}
	if hardware.Memory != "" {
		resReq[corev1.ResourceMemory] = resource.MustParse(hardware.Memory)
	}
	if hardware.EphemeralStorage != "" {
		resReq[corev1.ResourceEphemeralStorage] = resource.MustParse(hardware.EphemeralStorage)
	}
	if hardware.Gpu.ResourceName != "" && hardware.Gpu.Num != "" {
		resReq[corev1.ResourceName(hardware.Gpu.ResourceName)] = resource.MustParse(hardware.Gpu.Num)
	}
	return resReq, nodeSelector
}

// NewPersistentVolumeClaim creates a new k8s PVC with some default values set.
func (s *ServiceComponent) NewPersistentVolumeClaim(name string, ctx context.Context, cluster cluster.Cluster, hardware types.HardWare) error {
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
