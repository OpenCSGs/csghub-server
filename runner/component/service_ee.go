//go:build ee || saas

package component

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	lwsv1 "sigs.k8s.io/lws/api/leaderworkerset/v1"
)

var (
	LWSSuffix       string = "-lws"
	LWSLeaderSuffix string = "-leader-svc"
)

// RunServiceMultiHost will lerverage k8s lws to deploy multi-host service
// we will create lws first, then create knative service to proxy to lws
func (s *serviceComponentImpl) runServiceMultiHost(ctx context.Context, req types.SVCRequest) error {
	cluster, err := s.clusterPool.GetClusterByID(ctx, req.ClusterID)
	if err != nil {
		return fmt.Errorf("fail to get cluster, error %w ", err)
	}
	// check if the ksvc exists
	_, err = cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Get(ctx, req.SvcName, metav1.GetOptions{})
	if err == nil {
		slog.Warn("service already exists,delete it first", slog.String("srv_name", req.SvcName), slog.Any("image_id", req.ImageID))
		return fmt.Errorf("service %v already exists,delete it first. error: %w ", req.SvcName, err)
	}
	// create lws service
	lws, err := s.GenerateLWSService(ctx, *cluster, req)
	if err != nil {
		return fmt.Errorf("fail to generate leaderworkset service, %w ", err)
	}
	_, err = cluster.LWSClient.LeaderworkersetV1().LeaderWorkerSets(s.k8sNameSpace).Create(ctx, lws, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create lws deployment, error: %w", err)
	}
	slog.Info("create lws service success", slog.String("srv_name", req.SvcName))
	err = s.CreateLWSLeaderService(ctx, *cluster, req.SvcName)
	if err != nil {
		err2 := cluster.LWSClient.LeaderworkersetV1().LeaderWorkerSets(s.k8sNameSpace).Delete(ctx, req.SvcName+LWSSuffix, metav1.DeleteOptions{})
		if err2 != nil {
			return fmt.Errorf("failed to create lws leader service, error: %w, start to purge lws service, error: %w", err, err2)
		}
		return fmt.Errorf("failed to create lws leader service, error: %w", err)
	}
	//create knative service
	service, err := s.GenerateNginxService(ctx, cluster, req)
	//add label to service
	service.Annotations[KeyRunModeLabel] = ValueMultiHost
	if err != nil {
		return fmt.Errorf("fail to generate proxy service, %v ", err)
	}
	ksvc, err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		err2 := s.RemoveWorkset(ctx, *cluster, ksvc)
		if err2 != nil {
			return fmt.Errorf("failed to create proxy service, error: %w, failed to delete lws service, error: %w", err, err2)
		}
		return fmt.Errorf("failed to create proxy service, error: %w, req: %v", err, req)
	}

	// add a placeholder service

	newKS := &database.KnativeService{
		Code:       common.Deploying,
		Name:       req.SvcName,
		ClusterID:  req.ClusterID,
		Status:     corev1.ConditionUnknown,
		DeployID:   req.DeployID,
		UserUUID:   req.UserID,
		DeployType: req.DeployType,
		TaskID:     req.TaskId,
	}
	if err := s.addKServiceWithEvent(ctx, newKS); err != nil {
		return fmt.Errorf("failed to add kservice, error: %w", err)
	}

	return nil
}

func (s *serviceComponentImpl) GenerateNginxService(ctx context.Context, cluster *cluster.Cluster, req types.SVCRequest) (*v1.Service, error) {
	req.Hardware = types.HardWare{
		Cpu: types.CPU{
			Num: "2",
		},
	}
	service, err := s.generateService(ctx, cluster, req)
	containerImg := path.Join(s.modelDockerRegBase, "opencsghq/nginx:latest")
	service.Spec.Template.Spec.Containers[0].Image = containerImg
	appPort, ok := req.Env["port"]
	if !ok {
		return nil, fmt.Errorf("failed to find port from env")
	}
	//route to leader service
	proxyConfig := fmt.Sprintf(`server {
		listen %s;
		location / {
		  proxy_pass http://%s:%s;
		  proxy_set_header Host \$host;
		  proxy_set_header X-Real-IP \$remote_addr;
		  proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
		}
	  }`, appPort, req.SvcName+LWSLeaderSuffix, appPort)
	service.Spec.Template.Spec.Containers[0].Command = []string{
		"/bin/sh",
		"-c",
		fmt.Sprintf("cat > /etc/nginx/conf.d/default.conf <<EOF\n%s\nEOF\nnginx -g 'daemon off;'", proxyConfig),
	}
	service.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"curl",
					"-f",
					fmt.Sprintf("http://%s:%s/health", req.SvcName+LWSSuffix, appPort),
				},
			},
		},
		InitialDelaySeconds: 60,
		PeriodSeconds:       30,
	}
	return service, err
}

func (s *serviceComponentImpl) GenerateLWSService(ctx context.Context, cluster cluster.Cluster, request types.SVCRequest) (*lwsv1.LeaderWorkerSet, error) {

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
		if err != nil || appPort == 0 {
			return nil, fmt.Errorf("port is not valid number, error: %w", err)
		}
	}
	//append more environemt
	environments = s.GenerateLWSEnv(request.Hardware, environments, cluster)

	// knative service spec container port
	exposePorts := []corev1.ContainerPort{{
		ContainerPort: int32(appPort),
	}}
	// knative service spec resource requirement
	resources := corev1.ResourceRequirements{
		Limits:   resReq,
		Requests: resReq,
	}

	containerImg := request.ImageID
	// add prefix if image is not full path
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

	imagePullSecrets := []corev1.LocalObjectReference{
		{
			Name: s.imagePullSecret,
		},
	}

	volumes, volumeMounts, err := s.GenerateLWSVolumes(ctx, request, &cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to generate lws volumes, error: %w", err)
	}

	lws := &lwsv1.LeaderWorkerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.SvcName + LWSSuffix,
			Namespace: s.k8sNameSpace,
		},
		Spec: lwsv1.LeaderWorkerSetSpec{
			Replicas:      ptr.To(int32(request.MinReplica)),
			StartupPolicy: lwsv1.LeaderReadyStartupPolicy,
			LeaderWorkerTemplate: lwsv1.LeaderWorkerTemplate{
				RestartPolicy: lwsv1.NoneRestartPolicy,
				Size:          ptr.To(int32(request.Hardware.Replicas)),
				LeaderTemplate: &corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":  request.SvcName,
							"role": "leader",
							//A hack label for compatibility with Knative services
							// list pods will check this label to decide whether to list the pod
							"serving.knative.dev/service": request.SvcName,
							types.LogLabelTypeKey:         types.LogLabelDeploy,
						},
					},
					Spec: corev1.PodSpec{
						HostNetwork:      true,
						HostPID:          true,
						DNSPolicy:        corev1.DNSClusterFirstWithHostNet,
						NodeSelector:     nodeSelector,
						ImagePullSecrets: imagePullSecrets,
						Containers: []corev1.Container{
							{
								Name:            "leader",
								Image:           containerImg,
								ImagePullPolicy: corev1.PullAlways,
								Resources:       resources,
								VolumeMounts:    volumeMounts,
								Env:             environments,
								Ports:           exposePorts,
							},
						},
						Volumes: volumes,
					},
				},
				WorkerTemplate: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							//A hack label for compatibility with Knative services
							"serving.knative.dev/service": request.SvcName,
							types.LogLabelTypeKey:         types.LogLabelDeploy,
						},
					},
					Spec: corev1.PodSpec{
						HostNetwork: true,
						HostPID:     true,
						DNSPolicy:   corev1.DNSClusterFirstWithHostNet,
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "app",
													Operator: metav1.LabelSelectorOpIn,
													Values:   []string{request.SvcName},
												},
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "worker",
								Image:           containerImg,
								Resources:       resources,
								VolumeMounts:    volumeMounts,
								Env:             environments,
								ImagePullPolicy: corev1.PullAlways,
							},
						},
						Volumes: volumes,
					},
				},
			},
		},
	}

	return lws, nil
}

// generate volume mounts
func (s *serviceComponentImpl) GenerateLWSVolumes(ctx context.Context, request types.SVCRequest, cluster *cluster.Cluster) ([]corev1.Volume, []corev1.VolumeMount, error) {
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
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

	if cluster.StorageClass != "" {
		pvcName := request.UserID
		err := s.newPersistentVolumeClaim(pvcName, ctx, cluster, request.Hardware)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create persist volume, %w", err)
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
	return volumes, volumeMounts, nil
}

func (s *serviceComponentImpl) GenerateLWSEnv(hardware types.HardWare, environments []corev1.EnvVar, cluster cluster.Cluster) []corev1.EnvVar {

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
	}

	environments = append(environments, corev1.EnvVar{
		Name: "LWS_WORKER_INDEX",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.labels['leaderworkerset.sigs.k8s.io/worker-index']",
			},
		},
	})
	xpuNumber, err := common.GetXPUNumber(hardware)

	if err != nil {
		return environments
	}
	totalGPU := xpuNumber * hardware.Replicas
	if !s.isVariableExist(environments, "GPU_NUM") {
		environments = append(environments, corev1.EnvVar{Name: "GPU_NUM", Value: fmt.Sprintf("%d", xpuNumber)})
	}
	if !s.isVariableExist(environments, "TOTAL_GPU") {
		environments = append(environments, corev1.EnvVar{Name: "TOTAL_GPU", Value: fmt.Sprintf("%d", totalGPU)})
	}
	if !s.isVariableExist(environments, "NCCL_SOCKET_IFNAME") {
		environments = append(environments, corev1.EnvVar{Name: "NCCL_SOCKET_IFNAME", Value: cluster.NetworkInterface})
	}
	if !s.isVariableExist(environments, "GLOO_SOCKET_IFNAME") {
		environments = append(environments, corev1.EnvVar{Name: "GLOO_SOCKET_IFNAME", Value: cluster.NetworkInterface})
	}
	return environments
}

// check if the environments is exist
func (s *serviceComponentImpl) isVariableExist(env []corev1.EnvVar, name string) bool {
	for _, e := range env {
		if e.Name == name {
			return true
		}
	}
	return false
}

func (s *serviceComponentImpl) RemoveWorkset(ctx context.Context, cluster cluster.Cluster, ksvc *v1.Service) error {
	mulHost := ksvc.Annotations[KeyRunModeLabel]
	lwsName := ksvc.Name + LWSSuffix
	lwsLeaderSvcName := ksvc.Name + LWSLeaderSuffix
	if mulHost == ValueMultiHost {
		err := cluster.LWSClient.LeaderworkersetV1().LeaderWorkerSets(s.k8sNameSpace).Delete(ctx, lwsName, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete lws, error: %w", err)
		}
		err = cluster.Client.CoreV1().Services(s.k8sNameSpace).Delete(ctx, lwsLeaderSvcName, metav1.DeleteOptions{})
		return err
	}
	return nil
}

// update lws service to use leader app
func (s *serviceComponentImpl) CreateLWSLeaderService(ctx context.Context, cluster cluster.Cluster, srvName string) error {
	// Get the Service
	serviceName := srvName + LWSLeaderSuffix
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			// select leader pod only
			Selector: map[string]string{
				"leaderworkerset.sigs.k8s.io/name": srvName + LWSSuffix,
				"role":                             "leader",
			},
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       8000,
					TargetPort: intstr.FromInt(8000),
				},
			},
		},
	}
	_, err := cluster.Client.CoreV1().Services(s.k8sNameSpace).Create(ctx, service, metav1.CreateOptions{})
	return err
}
