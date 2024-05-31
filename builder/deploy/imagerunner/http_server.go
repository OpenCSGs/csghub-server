package imagerunner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type HttpServer struct {
	clusterPool        *cluster.ClusterPool
	spaceDockerRegBase string
	modelDockerRegBase string
	k8sNameSpace       string
	imagePullSecret    string
	env                *config.Config
}

func NewHttpServer(config *config.Config) (*HttpServer, error) {
	clusterPool, err := cluster.NewClusterPool()
	if err != nil {
		slog.Error("falied to build kubeconfig", "error", err)
		return nil, fmt.Errorf("failed to build kubeconfig,%w", err)
	}
	domainParts := strings.SplitN(config.Space.InternalRootDomain, ".", 2)
	return &HttpServer{
		spaceDockerRegBase: config.Space.DockerRegBase,
		modelDockerRegBase: config.Model.DockerRegBase,
		k8sNameSpace:       domainParts[0],
		imagePullSecret:    config.Space.ImagePullSecret,
		clusterPool:        clusterPool,
		env:                config,
	}, nil
}

func (s *HttpServer) Run(port int) error {
	router := gin.Default()
	router.Use(middleware.Log())

	router.POST("/:service/run", s.runService)
	router.PUT("/:service/update", s.updateService)
	router.POST("/:service/stop", s.stopService)
	router.GET("/:service/status", s.serviceStatus)
	router.GET("/:service/logs", s.serviceLogs)
	router.GET("/:service/logs/:pod_name", s.serviceLogsByPod)
	router.GET("/:service/info", s.getServiceInfo)
	router.GET("/status-all", s.serviceStatusAll)
	router.GET("/cluster/status", s.getClusterStatus)
	router.PUT("/cluster", s.updateCluster)
	router.GET("/:service/get", s.getServiceByName)
	router.GET("/:service/replica", s.getReplica)

	return router.Run(fmt.Sprintf(":%d", port))
}

func (s *HttpServer) runService(c *gin.Context) {
	var request struct {
		ImageID    string            `json:"image_id" binding:"required"`
		Hardware   types.HardWare    `json:"hardware,omitempty"`
		Env        map[string]string `json:"env,omitempty"`
		Annotation map[string]string `json:"annotation,omitempty"`
		DeployID   int64             `json:"deploy_id" binding:"required"`
		RepoType   string            `json:"repo_type"`
		MinReplica int               `json:"min_replica"`
		MaxReplica int               `json:"max_replica"`
		ClusterID  string            `json:"cluster_id"`
	}

	err := c.BindJSON(&request)
	if err != nil {
		slog.Error("runService get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	slog.Debug("Recv request", slog.Any("body", request))

	cluster, err := s.clusterPool.GetClusterByID(c, request.ClusterID)
	if err != nil {
		slog.Error("fail to get cluster ", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	srvName := s.getServiceNameFromRequest(c)
	// check if the ksvc exists
	_, err = cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Get(c.Request.Context(), srvName, metav1.GetOptions{})
	if err == nil {
		cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Delete(c, srvName, *metav1.NewDeleteOptions(0))
		slog.Info("service already exists,delete it first", slog.String("srv_name", srvName), slog.Any("image_id", request.ImageID))
	}

	annotations := request.Annotation

	environments := []corev1.EnvVar{}
	appPort := 0
	hardware := request.Hardware
	resReq, nodeSelector := GenerateResources(hardware)

	if request.Env != nil {
		// generate env
		for key, value := range request.Env {
			environments = append(environments, corev1.EnvVar{Name: key, Value: value})
		}

		// get app expose port from env with key=port
		val, ok := request.Env["port"]
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find port from env"})
			return
		}

		appPort, err = strconv.Atoi(val)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "port is not valid number"})
			return
		}
	}

	if appPort == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "app export port is not defined"})
		return
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

	annotations["deploy_id"] = strconv.FormatInt(request.DeployID, 10)

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

	slog.Debug("ksvc", slog.Any("knative service", service))

	// create ksvc
	_, err = cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Create(c, service, metav1.CreateOptions{})
	if err != nil {
		slog.Error("Failed to create service", "error", err, slog.Int64("deploy_id", request.DeployID),
			slog.String("image_id", request.ImageID),
			slog.String("srv_name", srvName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create service"})
		return
	}

	slog.Info("service created successfully", slog.String("srv_name", srvName), slog.Int64("deploy_id", request.DeployID))
	c.JSON(http.StatusOK, gin.H{"message": "Service created successfully"})
}

func (s *HttpServer) stopService(c *gin.Context) {
	var resp StopResponse
	var request = &StopRequest{}
	err := c.BindJSON(request)

	if err != nil {
		slog.Error("stopService get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cluster, err := s.clusterPool.GetClusterByID(c, request.ClusterID)
	if err != nil {
		slog.Error("fail to get cluster ", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	srvName := s.getServiceNameFromRequest(c)
	srv, err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).
		Get(c.Request.Context(), srvName, metav1.GetOptions{})
	if err != nil {
		k8serr := new(k8serrors.StatusError)
		if errors.As(err, &k8serr) {
			if k8serr.Status().Code == http.StatusNotFound {
				slog.Info("stop image skip,service not exist", slog.String("srv_name", srvName), slog.Any("k8s_err", k8serr))
				resp.Code = 0
				resp.Message = "skip,service not exist"
				c.JSON(http.StatusOK, nil)
				return
			}
		}
		slog.Error("stop image failed, cannot get service info", slog.String("srv_name", srvName), slog.Any("error", err),
			slog.String("srv_name", srvName))
		resp.Code = -1
		resp.Message = "failed to get service status"
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	if srv == nil {
		resp.Code = 0
		resp.Message = "service not exist"
		c.JSON(http.StatusOK, resp)
		return
	}

	err = cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Delete(c, srvName, *metav1.NewDeleteOptions(0))
	if err != nil {
		slog.Error("stop image failed, cannot delete service ", slog.String("srv_name", srvName), slog.Any("error", err),
			slog.String("srv_name", srvName))
		resp.Code = -1
		resp.Message = "failed to get service status"
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	slog.Info("service deleted", slog.String("srv_name", srvName))
	resp.Code = 0
	resp.Message = "service deleted"
	c.JSON(http.StatusOK, resp)
}

func (s *HttpServer) updateService(c *gin.Context) {
	var resp types.ModelUpdateResponse
	var request = &types.ModelUpdateRequest{}
	err := c.BindJSON(request)

	if err != nil {
		slog.Error("updateService get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cluster, err := s.clusterPool.GetClusterByID(c, request.ClusterID)
	if err != nil {
		slog.Error("fail to get cluster ", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	srvName := s.getServiceNameFromRequest(c)
	srv, err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).
		Get(c.Request.Context(), srvName, metav1.GetOptions{})
	if err != nil {
		k8serr := new(k8serrors.StatusError)
		if errors.As(err, &k8serr) {
			if k8serr.Status().Code == http.StatusNotFound {
				slog.Info("update service skip,service not exist", slog.String("srv_name", srvName), slog.Any("k8s_err", k8serr))
				resp.Code = 0
				resp.Message = "skip,service not exist"
				c.JSON(http.StatusOK, nil)
				return
			}
		}
		slog.Error("update service failed, cannot get service info", slog.String("srv_name", srvName), slog.Any("error", err),
			slog.String("srv_name", srvName))
		resp.Code = -1
		resp.Message = "failed to get service status"
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	if srv == nil {
		resp.Code = 0
		resp.Message = "service not exist"
		c.JSON(http.StatusOK, resp)
		return
	}
	// Update Image
	containerImg := path.Join(s.modelDockerRegBase, request.ImageID)
	srv.Spec.Template.Spec.Containers[0].Image = containerImg
	// Update env
	environments := []corev1.EnvVar{}
	if request.Env != nil {
		// generate env
		for key, value := range request.Env {
			environments = append(environments, corev1.EnvVar{Name: key, Value: value})
		}
		srv.Spec.Template.Spec.Containers[0].Env = environments
	}
	// Update CPU and Memory requests and limits
	hardware := request.Hardware
	resReq, _ := GenerateResources(hardware)
	resources := corev1.ResourceRequirements{
		Limits:   resReq,
		Requests: resReq,
	}
	srv.Spec.Template.Spec.Containers[0].Resources = resources
	// Update replica
	srv.Spec.Template.Annotations["autoscaling.knative.dev/min-scale"] = strconv.Itoa(request.MinReplica)
	srv.Spec.Template.Annotations["autoscaling.knative.dev/max-scale"] = strconv.Itoa(request.MaxReplica)

	_, err = cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Update(c, srv, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("failed to update service ", slog.String("srv_name", srvName), slog.Any("error", err),
			slog.String("srv_name", srvName))
		resp.Code = -1
		resp.Message = "failed to update service"
		c.JSON(http.StatusInternalServerError, resp)
		return
	}

	slog.Info("service updated", slog.String("srv_name", srvName))
	resp.Code = 0
	resp.Message = "service updated"
	c.JSON(http.StatusOK, resp)
}

func (s *HttpServer) serviceStatus(c *gin.Context) {
	var resp StatusResponse

	var request = &StatusRequest{}
	err := c.BindJSON(request)

	if err != nil {
		slog.Error("serviceStatus get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cluster, err := s.clusterPool.GetClusterByID(c, request.ClusterID)

	if err != nil {
		slog.Error("fail to get cluster ", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	srvName := s.getServiceNameFromRequest(c)
	srv, err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).
		Get(c.Request.Context(), srvName, metav1.GetOptions{})
	if err != nil {
		slog.Error("get image status failed, cannot get service info", slog.String("srv_name", srvName), slog.Any("error", err),
			slog.String("srv_name", srvName))
		resp.Code = common.Stopped
		resp.Message = "failed to get service status"
		c.JSON(http.StatusOK, resp)
		return
	}
	deployIDStr := srv.Annotations["deploy_id"]
	deployID, _ := strconv.ParseInt(deployIDStr, 10, 64)
	resp.DeployID = deployID

	// retrive pod list and status
	instList, err := s.getServicePodsWithStatus(c.Request.Context(), *cluster, srvName, s.k8sNameSpace)
	if err != nil {
		slog.Error("fail to get service pod name list", slog.Any("error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "fail to get service pod name list"})
		return
	}
	resp.Instances = instList

	if srv.IsFailed() {
		resp.Code = common.DeployFailed
		// read message of Ready
		resp.Message = srv.Status.GetCondition(v1.ServiceConditionReady).Message
		// append message of ConfigurationsReady
		srvConfigReady := srv.Status.GetCondition(v1.ServiceConditionConfigurationsReady)
		if srvConfigReady != nil {
			resp.Message += srvConfigReady.Message
		}
		slog.Info("get image status success", slog.String("srv_name", srvName), slog.Any("resp", resp))
		c.JSON(http.StatusOK, resp)
		return
	}

	if srv.IsReady() {
		podNames, err := s.getServicePods(c.Request.Context(), *cluster, srvName, s.k8sNameSpace, 1)
		if err != nil {
			slog.Error("get image status failed, cantnot get pods info", slog.String("srv_name", srvName), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "message": "unkown service status, failed to get pods"})
			return
		}
		if len(podNames) == 0 {
			resp.Code = common.Sleeping
			resp.Message = "service sleeping, no running pods"
			slog.Info("get image status success", slog.String("srv_name", srvName), slog.Any("resp", resp))
			c.JSON(http.StatusOK, resp)
			return
		}

		resp.Code = common.Running
		resp.Message = "service running"
		if srv.Status.URL != nil {
			slog.Info("knative endpoint", slog.Any("svc name", srvName), slog.Any("url", srv.Status.URL.URL().String()))
			resp.Endpoint = srv.Status.URL.URL().String()
		}

		slog.Info("get image status success", slog.String("srv_name", srvName), slog.Any("resp", resp))
		c.JSON(http.StatusOK, resp)
		return
	}

	// default to deploying status
	resp.Code = common.Deploying
	resp.Message = "service is not ready or failed"
	slog.Info("get image status success, service is not ready or failed", slog.String("srv_name", srvName), slog.Any("resp", resp))
	c.JSON(http.StatusOK, resp)
}

func (s *HttpServer) serviceLogs(c *gin.Context) {
	var request = &LogsRequest{}
	err := c.BindJSON(request)

	if err != nil {
		slog.Error("serviceLogs get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cluster, err := s.clusterPool.GetClusterByID(c, request.ClusterID)
	if err != nil {
		slog.Error("fail to get cluster ", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	srvName := s.getServiceNameFromRequest(c)
	podNames, err := s.getServicePods(c.Request.Context(), *cluster, srvName, s.k8sNameSpace, 1)
	if err != nil {
		slog.Error("failed to read image logs, cannot get pods info", slog.Any("error", err), slog.String("srv_name", srvName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get pods info"})
		return
	}
	if len(podNames) == 0 {
		slog.Error("failed to read image logs, no running pods", slog.String("srv_name", srvName))
		c.JSON(http.StatusNotFound, gin.H{"error": "no running pods, service maybe sleeping"})
		return
	}
	s.getLogsByPod(c, *cluster, podNames[0], srvName)
}

func (s *HttpServer) serviceLogsByPod(c *gin.Context) {
	var request = &ServiceRequest{}
	err := c.BindJSON(request)

	if err != nil {
		slog.Error("serviceLogs get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cluster, err := s.clusterPool.GetClusterByID(c, request.ClusterID)
	if err != nil {
		slog.Error("fail to get cluster ", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	srvName := s.getServiceNameFromRequest(c)
	podName := s.getPodNameFromRequest(c)
	s.getLogsByPod(c, *cluster, podName, srvName)
}

func (s *HttpServer) getLogsByPod(c *gin.Context, cluster cluster.Cluster, podName string, srvName string) {

	logs := cluster.Client.CoreV1().Pods(s.k8sNameSpace).GetLogs(podName, &corev1.PodLogOptions{
		Container: "user-container",
		Follow:    true,
	})
	stream, err := logs.Stream(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open stream"})
		return
	}
	defer stream.Close()

	// c.Header("Content-Type", "text/event-stream")
	c.Header("Content-Type", "text/plain")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Writer.WriteHeader(http.StatusOK)
	buf := make([]byte, 32*1024)

	pod, err := cluster.Client.CoreV1().Pods(s.k8sNameSpace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		slog.Error("fail to get pod ", slog.Any("error", err), slog.String("pod name", podName))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if pod.Status.Phase == "Pending" {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "PodScheduled" && condition.Status == "False" {
				message := fmt.Sprintf("Pod is pending due to reason: %s, message: %s", condition.Reason, condition.Message)
				c.Writer.Write([]byte(message))
				c.Writer.Flush()
				c.JSON(http.StatusBadRequest, gin.H{"error": message})
				return
			}
		}
	}

	for {
		select {
		case <-c.Request.Context().Done():
			slog.Info("logs request context done", slog.Any("error", c.Request.Context().Err()))
			return
		default:
			n, err := stream.Read(buf)
			if err != nil {
				slog.Error("read pod logs failed", slog.Any("error", err), slog.String("srv_name", srvName))
				break
			}

			if n > 0 {
				c.Writer.Write(buf[:n])
				c.Writer.Flush()
				slog.Info("send pod logs", slog.String("srv_name", srvName), slog.String("srv_name", srvName), slog.Int("len", n), slog.String("log", string(buf[:n])))
			}
			// c.Writer.WriteString("test messagetest messagetest messagetest messagetest messagetest messagetest messagetest messagetest message")
			// c.Writer.Flush()
		}
		time.Sleep(5 * time.Second)
	}
}

func (s *HttpServer) serviceStatusAll(c *gin.Context) {
	allStatus := make(map[string]*StatusResponse)
	for index := range s.clusterPool.Clusters {
		cluster := s.clusterPool.Clusters[index]
		services, err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).
			List(c.Request.Context(), metav1.ListOptions{})
		if err != nil {
			slog.Error("get image status all failed, cannot get service infos", slog.Any("error", err))
			c.Status(http.StatusInternalServerError)
			return
		}

		for _, srv := range services.Items {
			deployIDStr := srv.Annotations["deploy_id"]
			deployID, _ := strconv.ParseInt(deployIDStr, 10, 64)
			status := &StatusResponse{
				DeployID: deployID,
			}
			allStatus[srv.Name] = status
			if srv.IsFailed() {
				status.Code = common.DeployFailed
				continue
			}

			if srv.IsReady() {
				podNames, err := s.getServicePods(c.Request.Context(), cluster, srv.Name, s.k8sNameSpace, 1)
				if err != nil {
					slog.Error("get image status failed, cannot get pods info", slog.Any("error", err))
					status.Code = common.Running
					continue
				}

				if len(podNames) == 0 {
					status.Code = common.Sleeping
					continue
				}

				status.Code = common.Running
				continue
			}

			// default to deploying
			status.Code = common.Deploying
		}
	}

	c.JSON(http.StatusOK, allStatus)
}

func (s *HttpServer) getServicePods(ctx context.Context, cluster cluster.Cluster, srvName string, namespace string, limit int64) ([]string, error) {
	labelSelector := fmt.Sprintf("serving.knative.dev/service=%s", srvName)
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
		podNames = append(podNames, pod.Name)
	}

	return podNames, nil
}

func (s *HttpServer) getClusterStatus(c *gin.Context) {
	clusterRes := []CluserResponse{}
	for index := range s.clusterPool.Clusters {
		cls := s.clusterPool.Clusters[index]
		cInfo, _ := s.clusterPool.ClusterStore.ByClusterConfig(c.Request.Context(), cls.ID)
		if !cInfo.Enable {
			continue
		}
		nodes, err := cluster.GetNodeResources(cls.Client, s.env)
		if err == nil {
			clusterInfo := CluserResponse{}
			clusterInfo.Nodes = nodes
			clusterInfo.Region = cInfo.Region
			clusterInfo.Zone = cInfo.Zone
			clusterInfo.Provider = cInfo.Provider
			clusterInfo.ClusterID = cInfo.ClusterID
			clusterInfo.ClusterName = fmt.Sprintf("cluster%d", index)
			clusterInfo.Nodes = nodes
			clusterRes = append(clusterRes, clusterInfo)
		}

	}
	c.JSON(http.StatusOK, clusterRes)
}

func (s *HttpServer) getServiceNameFromRequest(c *gin.Context) string {
	return c.Params.ByName("service")
}

func (s *HttpServer) getPodNameFromRequest(c *gin.Context) string {
	return c.Params.ByName("pod_name")
}

func (s *HttpServer) getServiceByName(c *gin.Context) {
	var resp StatusResponse
	var request = &CheckRequest{}
	err := c.BindJSON(request)
	if err != nil {
		slog.Error("fail to parse input parameters", slog.Any("error", err))
		resp.Code = -1
		resp.Message = "fail to parse input parameters"
		c.JSON(http.StatusOK, resp)
		return
	}
	cluster, err := s.clusterPool.GetClusterByID(c, request.ClusterID)
	if err != nil {
		slog.Error("fail to get cluster config", slog.Any("error", err))
		resp.Code = -1
		resp.Message = "fail to get cluster config"
		c.JSON(http.StatusOK, resp)
		return
	}
	srvName := s.getServiceNameFromRequest(c)
	srv, err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Get(c.Request.Context(), srvName, metav1.GetOptions{})
	if err != nil {
		k8serr := new(k8serrors.StatusError)
		if errors.As(err, &k8serr) {
			if k8serr.Status().Code == http.StatusNotFound {
				// service not exist
				resp.Code = 0
				resp.Message = "service not exist"
				c.JSON(http.StatusOK, resp)
				return
			}
		}
		// get service with error
		slog.Error("fail to get service with error", slog.Any("error", err))
		resp.Code = -1
		resp.Message = "fail to get service"
		c.JSON(http.StatusOK, resp)
		return
	}

	if srv == nil {
		// service not exist
		resp.Code = 0
		resp.Message = "service not exist"
		c.JSON(http.StatusOK, resp)
		return
	}

	// service exist
	deployIDStr := srv.Annotations[types.ResDeployID]
	deployID, _ := strconv.ParseInt(deployIDStr, 10, 64)
	resp.DeployID = deployID
	resp.Code = 1
	resp.Message = srvName
	if srv.Status.URL != nil {
		resp.Endpoint = srv.Status.URL.URL().String()
	}
	c.JSON(http.StatusOK, resp)
}

func (s *HttpServer) getReplica(c *gin.Context) {
	var resp ReplicaResponse
	var request = &StatusRequest{}
	err := c.BindJSON(request)
	if err != nil {
		slog.Error("fail to parse input parameters", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to parse input parameters"})
		return
	}
	cluster, err := s.clusterPool.GetClusterByID(c, request.ClusterID)
	if err != nil {
		slog.Error("fail to get cluster config", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to get cluster config"})
		return
	}
	srvName := s.getServiceNameFromRequest(c)
	srv, err := cluster.KnativeClient.ServingV1().Services(s.k8sNameSpace).Get(c.Request.Context(), srvName, metav1.GetOptions{})
	if err != nil {
		// get service with error
		slog.Error("fail to get service", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to get service"})
		return
	}

	if srv == nil {
		// service not exist
		slog.Error("service not exist")
		c.JSON(http.StatusNotFound, gin.H{"error": "service not exist"})
		return
	}
	// revisionName := srv.Status.LatestReadyRevisionName
	revisionName := srv.Status.LatestCreatedRevisionName
	if len(revisionName) < 1 {
		slog.Error("fail to get latest created revision")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to get latest created revision"})
		return
	}
	revision, err := cluster.KnativeClient.ServingV1().Revisions(s.k8sNameSpace).Get(c.Request.Context(), revisionName, metav1.GetOptions{})
	if err != nil {
		slog.Error("fail to get revision with error", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to get revision with error"})
		return
	}

	if revision == nil {
		slog.Error("revision not exist")
		c.JSON(http.StatusNotFound, gin.H{"error": "revision not exist"})
		return
	}
	instList, err := s.getServicePodsWithStatus(c.Request.Context(), *cluster, srvName, s.k8sNameSpace)
	if err != nil {
		slog.Error("fail to get service pod name list", slog.Any("error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": "fail to get service pod name list"})
		return
	}

	// revision exist
	deployIDStr := srv.Annotations[types.ResDeployID]
	deployID, _ := strconv.ParseInt(deployIDStr, 10, 64)
	resp.DeployID = deployID
	resp.Code = 1
	resp.Message = srvName
	resp.ActualReplica = int(*revision.Status.ActualReplicas)
	resp.DesiredReplica = int(*revision.Status.DesiredReplicas)
	resp.Instances = instList
	c.JSON(http.StatusOK, resp)
}

func (s *HttpServer) updateCluster(c *gin.Context) {
	var resp UpdateClusterResponse
	var request = &database.ClusterInfo{}
	err := c.BindJSON(request)
	if err != nil {
		slog.Error("fail to parse input parameters", slog.Any("error", err))
		resp.Code = -1
		resp.Message = "fail to parse input parameters"
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	err = s.clusterPool.ClusterStore.Update(c, *request)
	if err != nil {
		slog.Error("fail to update cluster", slog.Any("error", err))
		resp.Code = -1
		resp.Message = "fail to update cluster"
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	resp.Code = 0
	resp.Message = "succeed to update cluster"
	c.JSON(http.StatusOK, resp)
}

func (s *HttpServer) getServiceInfo(c *gin.Context) {
	var resp ServiceInfoResponse
	var request = &ServiceRequest{}
	err := c.BindJSON(request)
	if err != nil {
		slog.Error("fail to parse input parameters", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to parse input parameters"})
		return
	}
	cluster, err := s.clusterPool.GetClusterByID(c, request.ClusterID)
	if err != nil {
		slog.Error("fail to get cluster config", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to get cluster config"})
		return
	}

	srvName := s.getServiceNameFromRequest(c)
	podNames, err := s.getServicePods(c.Request.Context(), *cluster, srvName, s.k8sNameSpace, -1)
	if err != nil {
		slog.Error("failed to read image logs, cannot get pods info", slog.Any("error", err), slog.String("srv_name", srvName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get pods info"})
		return
	}
	resp.PodNames = podNames
	resp.ServiceName = srvName
	c.JSON(http.StatusOK, resp)
}

func (s *HttpServer) getServicePodsWithStatus(ctx context.Context, cluster cluster.Cluster, srvName string, namespace string) ([]types.Instance, error) {
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
				Status: getPodStatus(pod.Status.ContainerStatuses),
			},
		)
	}

	return podInstances, nil
}

func getPodStatus(containerStatuses []corev1.ContainerStatus) string {
	// get knative user-container status
	status := string(corev1.PodFailed)
	for _, container := range containerStatuses {
		if container.Name == "user-container" {
			if container.Ready && container.State.Running != nil {
				status = string(corev1.PodRunning)
				return status
			}
			if !container.Ready && container.State.Waiting != nil {
				status = container.State.Waiting.Reason
				return status
			}
			if !container.Ready && container.State.Terminated != nil {
				status = container.State.Terminated.Reason
				return status
			}
		}
	}
	return status
}

func GenerateResources(hardware types.HardWare) (map[corev1.ResourceName]resource.Quantity, map[string]string) {
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
