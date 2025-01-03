package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/runner/component"
)

type K8sHandler struct {
	clusterPool        *cluster.ClusterPool
	k8sNameSpace       string
	modelDockerRegBase string
	env                *config.Config
	serviceCompoent    component.ServiceComponent
}

func NewK8sHandler(config *config.Config, clusterPool *cluster.ClusterPool) (*K8sHandler, error) {
	domainParts := strings.SplitN(config.Space.InternalRootDomain, ".", 2)
	serviceComponent := component.NewServiceComponent(config, clusterPool)
	go serviceComponent.RunInformer()
	return &K8sHandler{
		k8sNameSpace:       domainParts[0],
		clusterPool:        clusterPool,
		env:                config,
		serviceCompoent:    serviceComponent,
		modelDockerRegBase: config.Model.DockerRegBase,
	}, nil
}

func (s *K8sHandler) RunService(c *gin.Context) {
	request := &types.SVCRequest{}
	err := c.BindJSON(&request)
	if err != nil {
		slog.Error("runService get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	srvName := s.getServiceNameFromRequest(c)
	request.SrvName = srvName
	err = s.serviceCompoent.RunService(c.Request.Context(), *request)
	if err != nil {
		slog.Error("fail to run service", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	slog.Info("service created successfully", slog.String("srv_name", srvName), slog.Int64("deploy_id", request.DeployID))
	c.JSON(http.StatusOK, gin.H{"message": "Service created successfully"})
}

func (s *K8sHandler) StopService(c *gin.Context) {

	var request = &types.StopRequest{}
	err := c.BindJSON(request)

	if err != nil {
		slog.Error("stopService get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	srvName := s.getServiceNameFromRequest(c)
	request.SvcName = srvName
	resp, err := s.serviceCompoent.StopService(c.Request.Context(), *request)
	if err != nil {
		slog.Error("failed to stop service", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("service deleted", slog.String("srv_name", srvName))
	resp.Code = 0
	resp.Message = "service deleted"
	c.JSON(http.StatusOK, resp)
}

func (s *K8sHandler) UpdateService(c *gin.Context) {

	var request = &types.ModelUpdateRequest{}
	err := c.BindJSON(request)

	if err != nil {
		slog.Error("updateService get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	srvName := s.getServiceNameFromRequest(c)
	request.SvcName = srvName
	resp, err := s.serviceCompoent.UpdateService(c.Request.Context(), *request)
	if err != nil {
		slog.Error("failed to update service", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	slog.Info("service updated", slog.String("srv_name", srvName))
	resp.Code = 0
	resp.Message = "service updated"
	c.JSON(http.StatusOK, resp)
}

func (s *K8sHandler) ServiceStatus(c *gin.Context) {
	var request = &types.StatusRequest{}
	err := c.BindJSON(request)
	if err != nil {
		slog.Error("serviceStatus get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	srvName := s.getServiceNameFromRequest(c)
	resp, err := s.serviceCompoent.GetServiceByName(c.Request.Context(), srvName, request.ClusterID)
	if err != nil {
		slog.Error("failed to get service", slog.Any("error", err), slog.String("srv_name", srvName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get service"})
	}
	c.JSON(http.StatusOK, resp)
}

func (s *K8sHandler) ServiceLogs(c *gin.Context) {
	var request = &types.LogsRequest{}
	err := c.BindJSON(request)

	if err != nil {
		slog.Error("get bad request", slog.Any("error", err), slog.Any("req", request))
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
	podNames, err := s.serviceCompoent.GetServicePods(c.Request.Context(), *cluster, srvName, s.k8sNameSpace, 1)
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
	s.GetLogsByPod(c, *cluster, podNames[0], srvName)
}

func (s *K8sHandler) ServiceLogsByPod(c *gin.Context) {
	var request = &types.ServiceRequest{}
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
	s.GetLogsByPod(c, *cluster, podName, srvName)
}

func (s *K8sHandler) GetLogsByPod(c *gin.Context, cluster cluster.Cluster, podName string, srvName string) {

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
				_, err = c.Writer.Write([]byte(message))
				if err != nil {
					slog.Error("write data failed", "error", err)
				}
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
			if n == 0 {
				time.Sleep(5 * time.Second)
			}

			if n > 0 {
				_, err = c.Writer.Write(buf[:n])
				if err != nil {
					slog.Error("write data failed", "error", err)
				}
				c.Writer.Flush()
				slog.Info("send pod logs", slog.String("srv_name", srvName), slog.String("srv_name", srvName), slog.Int("len", n))
			}
		}

	}
}

func (s *K8sHandler) ServiceStatusAll(c *gin.Context) {
	allStatus, err := s.serviceCompoent.GetAllServiceStatus(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, allStatus)
}

func (s *K8sHandler) GetClusterInfo(c *gin.Context) {
	clusterRes := []types.CluserResponse{}
	for index := range s.clusterPool.Clusters {
		cls := s.clusterPool.Clusters[index]
		cInfo, err := s.clusterPool.ClusterStore.ByClusterConfig(c.Request.Context(), cls.CID)
		if err != nil {
			slog.Error("get cluster info failed", slog.Any("error", err))
			continue
		}
		clusterInfo := types.CluserResponse{}
		clusterInfo.Region = cInfo.Region
		clusterInfo.Zone = cInfo.Zone
		clusterInfo.Provider = cInfo.Provider
		clusterInfo.ClusterID = cInfo.ClusterID
		clusterInfo.ClusterName = fmt.Sprintf("cluster%d", index)
		clusterRes = append(clusterRes, clusterInfo)

	}
	c.JSON(http.StatusOK, clusterRes)
}

func (s *K8sHandler) GetClusterInfoByID(c *gin.Context) {
	clusterId := c.Params.ByName("id")
	cInfo, _ := s.clusterPool.ClusterStore.ByClusterID(c.Request.Context(), clusterId)
	clusterInfo := types.CluserResponse{}
	clusterInfo.Region = cInfo.Region
	clusterInfo.Zone = cInfo.Zone
	clusterInfo.Provider = cInfo.Provider
	clusterInfo.ClusterID = cInfo.ClusterID
	clusterInfo.StorageClass = cInfo.StorageClass
	client, err := s.clusterPool.GetClusterByID(c.Request.Context(), clusterId)
	if err != nil {
		slog.Error("fail to get cluster", slog.Any("error", err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	nodes, err := cluster.GetNodeResources(client.Client, s.env)
	if err == nil {
		clusterInfo.Nodes = nodes
	}

	c.JSON(http.StatusOK, clusterInfo)
}

func (s *K8sHandler) getServiceNameFromRequest(c *gin.Context) string {
	return c.Params.ByName("service")
}

func (s *K8sHandler) getPodNameFromRequest(c *gin.Context) string {
	return c.Params.ByName("pod_name")
}

func (s *K8sHandler) GetServiceByName(c *gin.Context) {
	var resp types.StatusResponse
	var request = &types.CheckRequest{}
	err := c.BindJSON(request)
	if err != nil {
		slog.Error("fail to parse input parameters", slog.Any("error", err))
		resp.Code = -1
		resp.Message = "fail to parse input parameters"
		c.JSON(http.StatusOK, resp)
		return
	}
	srvName := s.getServiceNameFromRequest(c)
	svc, err := s.serviceCompoent.GetServiceByName(c.Request.Context(), srvName, request.ClusterID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		resp.Code = -1
		resp.Message = "fail to get service"
		c.JSON(http.StatusOK, resp)
		return
	}
	if svc == nil {
		// service not exist
		resp.Code = common.Stopped
		resp.Message = "service not exist"
		c.JSON(http.StatusOK, resp)
		return
	}
	// service exist
	resp.DeployID = svc.DeployID
	resp.Code = svc.Code
	resp.Message = srvName
	resp.Endpoint = svc.Endpoint
	resp.Instances = svc.Instances
	c.JSON(http.StatusOK, resp)
}

func (s *K8sHandler) GetReplica(c *gin.Context) {
	var resp types.ReplicaResponse
	var request = &types.StatusRequest{}
	err := c.BindJSON(request)
	if err != nil {
		slog.Error("fail to parse input parameters", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to parse input parameters"})
		return
	}
	srvName := s.getServiceNameFromRequest(c)
	svc, err := s.serviceCompoent.GetServiceByName(c.Request.Context(), srvName, request.ClusterID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("fail to get service", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to get service"})
		return
	}
	if svc == nil {
		// service not exist
		slog.Error("service not exist")
		c.JSON(http.StatusNotFound, gin.H{"error": "service not exist"})
		return
	}

	// revision exist
	resp.Code = 1
	resp.Message = srvName
	resp.ActualReplica = svc.ActualReplica
	resp.DesiredReplica = svc.DesiredReplica
	resp.Instances = svc.Instances
	c.JSON(http.StatusOK, resp)
}

func (s *K8sHandler) UpdateCluster(c *gin.Context) {
	var resp types.UpdateClusterResponse
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

func (s *K8sHandler) PurgeService(c *gin.Context) {
	var resp = &types.PurgeResponse{}
	var request = &types.PurgeRequest{}
	err := c.BindJSON(request)
	if err != nil {
		slog.Error("fail to parse input parameters", slog.Any("error", err))
		resp.Code = -1
		resp.Message = "fail to parse cluster id"
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	srvName := s.getServiceNameFromRequest(c)
	request.SvcName = srvName
	resp, err = s.serviceCompoent.PurgeService(c.Request.Context(), *request)
	if err != nil {
		slog.Error("fail to purge service", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	slog.Info("service deleted.", slog.String("srv_name", srvName))
	resp.Code = 0
	resp.Message = "succeed to clean up service"
	c.JSON(http.StatusOK, resp)
}

func (s *K8sHandler) GetServiceInfo(c *gin.Context) {
	var request = &types.ServiceRequest{}
	err := c.BindJSON(request)
	if err != nil {
		slog.Error("fail to parse input parameters", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to parse input parameters"})
		return
	}

	srvName := s.getServiceNameFromRequest(c)
	request.ServiceName = srvName
	resp, err := s.serviceCompoent.GetServiceInfo(c.Request.Context(), *request)
	if err != nil {
		slog.Error("fail to get service info", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to get service info"})
		return
	}
	c.JSON(http.StatusOK, resp)
}
