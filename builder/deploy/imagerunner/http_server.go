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
	clusterPool     *cluster.ClusterPool
	dockerRegBase   string
	k8sNameSpace    string
	imagePullSecret string
	deployStore     *database.DeployTaskStore
	env             *config.Config
}

type CommonRequest struct {
	SpaceID   int64  `json:"space_id"`
	OrgName   string `json:"org_name"`
	SpaceName string `json:"space_name"`
}

func NewHttpServer(config *config.Config) (*HttpServer, error) {
	clusterPool, err := cluster.NewClusterPool()
	if err != nil {
		slog.Error("falied to build kubeconfig", "error", err)
		return nil, fmt.Errorf("failed to build kubeconfig,%w", err)
	}
	domainParts := strings.SplitN(config.Space.InternalRootDomain, ".", 2)
	return &HttpServer{
		dockerRegBase:   config.Space.DockerRegBase,
		k8sNameSpace:    domainParts[0],
		imagePullSecret: config.Space.ImagePullSecret,
		deployStore:     database.NewDeployTaskStore(),
		clusterPool:     clusterPool,
		env:             config,
	}, nil
}

func (s *HttpServer) Run(port int) error {
	router := gin.Default()
	router.Use(middleware.Log())

	router.POST("/:service/run", s.runService)
	router.POST("/:service/stop", s.stopService)
	router.GET("/:service/status", s.serviceStatus)
	router.GET("/:service/logs", s.serviceLogs)
	router.GET("/status-all", s.serviceStatusAll)
	router.GET("/cluster/status", s.getClusterStatus)

	return router.Run(fmt.Sprintf(":%d", port))
}

func (s *HttpServer) runService(c *gin.Context) {
	var request struct {
		ImageID    string            `json:"image_id" binding:"required"`
		Hardware   types.HardWare    `json:"hardware,omitempty"`
		Env        map[string]string `json:"env,omitempty"`
		DeployID   int64             `json:"deploy_id" binding:"required"`
		DeployType string            `json:"deploy_type" binding:"required"`
		ClusterId  string            `json:"cluster_id" binding:"required"`
	}

	err := c.BindJSON(&request)
	if err != nil {
		slog.Error("runService get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	slog.Debug("Recv request", slog.Any("body", request))

	cluster, err := s.clusterPool.GetClusterByID(request.ClusterId)
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

	nodeSelector := make(map[string]string)
	resources := corev1.ResourceRequirements{}
	resReq := make(map[corev1.ResourceName]resource.Quantity)
	environments := []corev1.EnvVar{}
	appPort := 0

	hardware := request.Hardware
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
	resources = corev1.ResourceRequirements{
		Limits:   resReq,
		Requests: resReq,
	}

	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      srvName,
			Namespace: s.k8sNameSpace,
			Annotations: map[string]string{
				"deploy_id": strconv.FormatInt(request.DeployID, 10),
			},
		},
		Spec: v1.ServiceSpec{
			ConfigurationSpec: v1.ConfigurationSpec{
				Template: v1.RevisionTemplateSpec{
					Spec: v1.RevisionSpec{
						PodSpec: corev1.PodSpec{
							NodeSelector: nodeSelector,
							Containers: []corev1.Container{{
								// TODO: docker registry url + image id
								// Image: "ghcr.io/knative/helloworld-go:latest",
								Image:     path.Join(s.dockerRegBase, request.ImageID),
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

	deploy, _ := s.deployStore.GetSpaceLatestDeploy(c, request.SpaceID)
	cluster, err := s.clusterPool.GetClusterByID(deploy.ClusterID)
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

func (s *HttpServer) serviceStatus(c *gin.Context) {
	var resp StatusResponse

	var request = &StatusRequest{}
	err := c.BindJSON(request)

	if err != nil {
		slog.Error("serviceStatus get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deploy, _ := s.deployStore.GetSpaceLatestDeploy(c, request.SpaceID)
	cluster, err := s.clusterPool.GetClusterByID(deploy.ClusterID)

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
		podNames, err := s.getServicePods(c.Request.Context(), *cluster, srvName, s.k8sNameSpace)
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
	deploy, _ := s.deployStore.GetSpaceLatestDeploy(c, request.SpaceID)
	cluster, err := s.clusterPool.GetClusterByID(deploy.ClusterID)
	if err != nil {
		slog.Error("fail to get cluster ", slog.Any("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	srvName := s.getServiceNameFromRequest(c)
	podNames, err := s.getServicePods(c.Request.Context(), *cluster, srvName, s.k8sNameSpace)
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
	podName := podNames[0]

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

	closeNotify := c.Writer.CloseNotify()

	buf := make([]byte, 1024)
	for {
		select {
		case <-closeNotify:
			// slog.Info("client disconnect from logs api", slog.String("image_id", imageID))
			slog.Info("client disconnect from logs api")
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
				podNames, err := s.getServicePods(c.Request.Context(), cluster, srv.Name, s.k8sNameSpace)
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

func (s *HttpServer) getServicePods(ctx context.Context, cluster cluster.Cluster, srvName string, namespace string) ([]string, error) {
	labelSelector := fmt.Sprintf("serving.knative.dev/service=%s", srvName)
	// Get the list of Pods based on the label selector
	pods, err := cluster.Client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         1,
	})
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
	clusterRes := []CluserInfo{}
	for index := range s.clusterPool.Clusters {
		cls := s.clusterPool.Clusters[index]
		nodes, err := cluster.GetNodeResources(cls.Client, s.env)
		if err == nil {
			clusterInfo := CluserInfo{}
			clusterInfo.Nodes = nodes
			clusterInfo.ClusterID = cls.ID
			clusterInfo.ClusterName = fmt.Sprintf("cluster%d", index)
			clusterInfo.Nodes = nodes
			for _, node := range nodes {
				clusterInfo.ClusterRegion = node.Region
				break
			}
			clusterRes = append(clusterRes, clusterInfo)
		}

	}
	c.JSON(http.StatusOK, clusterRes)
}

func (s *HttpServer) getServiceNameFromRequest(c *gin.Context) string {
	return c.Params.ByName("service")
}
