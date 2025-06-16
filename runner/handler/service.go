package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/builder/deploy/cluster"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	rcommon "opencsg.com/csghub-server/runner/common"
	"opencsg.com/csghub-server/runner/component"
	rTypes "opencsg.com/csghub-server/runner/types"
)

type K8sHandler struct {
	clusterPool        *cluster.ClusterPool
	k8sNameSpace       string
	modelDockerRegBase string
	env                *config.Config
	serviceComponent   component.ServiceComponent
}

func NewK8sHandler(config *config.Config, clusterPool *cluster.ClusterPool) (*K8sHandler, error) {
	domainParts := strings.SplitN(config.Space.InternalRootDomain, ".", 2)
	serviceComponent := component.NewServiceComponent(config, clusterPool)
	go serviceComponent.RunInformer()
	return &K8sHandler{
		k8sNameSpace:       domainParts[0],
		clusterPool:        clusterPool,
		env:                config,
		serviceComponent:   serviceComponent,
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
	svcName := s.getServiceNameFromRequest(c)
	request.SvcName = svcName
	err = s.serviceComponent.RunService(c.Request.Context(), *request)
	if err != nil {
		slog.Error("fail to run service", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	slog.Info("service created successfully", slog.String("svc_name", svcName), slog.Int64("deploy_id", request.DeployID))
	c.JSON(http.StatusOK, gin.H{"message": "Service created successfully"})
}

func (s *K8sHandler) StopService(c *gin.Context) {
	var request = &types.StopRequest{}
	err := c.BindJSON(request)

	if err != nil {
		slog.Error("stop service get bad request", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	svcName := s.getServiceNameFromRequest(c)
	request.SvcName = svcName
	resp, err := s.serviceComponent.StopService(c.Request.Context(), *request)
	if err != nil {
		slog.Error("failed to stop service", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("service deleted by request", slog.Any("req", request))
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
	svcName := s.getServiceNameFromRequest(c)
	request.SvcName = svcName
	resp, err := s.serviceComponent.UpdateService(c.Request.Context(), *request)
	if err != nil {
		slog.Error("failed to update service", slog.Any("error", err), slog.Any("req", request))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	slog.Info("service updated", slog.String("svc_name", svcName))
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
	svcName := s.getServiceNameFromRequest(c)
	resp, err := s.serviceComponent.GetServiceByName(c.Request.Context(), svcName, request.ClusterID)
	if err != nil {
		if err == sql.ErrNoRows {
			resp = &types.StatusResponse{}
			resp.Code = common.Stopped
			resp.Message = "service was deleted"
			c.JSON(http.StatusOK, resp)
			return
		}
		slog.Error("failed to get service", slog.Any("error", err), slog.String("svc_name", svcName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get service"})
		return
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
	svcName := s.getServiceNameFromRequest(c)
	podNames, err := s.serviceComponent.GetServicePods(c.Request.Context(), *cluster, svcName, s.k8sNameSpace, 1)
	if err != nil {
		slog.Error("failed to read image logs, cannot get pods info", slog.Any("error", err), slog.String("svc_name", svcName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get pods info"})
		return
	}
	podName := ""
	if len(podNames) > 0 {
		podName = podNames[0]
	}
	s.getLogsByPod(c, *cluster, podName, svcName)
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
	svcName := s.getServiceNameFromRequest(c)
	podName := s.getPodNameFromRequest(c)
	s.getLogsByPod(c, *cluster, podName, svcName)
}

func (s *K8sHandler) getLogsByPod(c *gin.Context, cluster cluster.Cluster, podName string, svcName string) {
	var (
		exist bool
		err   error
	)
	if len(podName) > 0 {
		exist, err = s.serviceComponent.PodExist(c.Request.Context(), cluster, podName)
		if err != nil {
			slog.Error("check pod existence", slog.Any("namespace", s.k8sNameSpace), slog.Any("pod-name", podName),
				slog.Any("clusterID", cluster.ID), slog.Any("error", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check pod existence"})
			return
		}
	}

	if exist {
		s.readPodLogsFromCluster(c, cluster, podName, svcName)
	} else {
		s.readPodLogsFromDB(c, cluster, podName, svcName)
	}
}

func (s *K8sHandler) readPodLogsFromDB(c *gin.Context, cluster cluster.Cluster, podName, svcName string) {
	slog.Debug("read pod logs from db", slog.Any("namespace", s.k8sNameSpace), slog.Any("pod-name", podName),
		slog.Any("svcname", svcName), slog.Any("clusterID", cluster.ID))
	logs, err := s.serviceComponent.GetPodLogsFromDB(c.Request.Context(), cluster, podName, svcName)
	if err != nil {
		slog.Error("failed to read pod logs from db",
			slog.Any("namespace", s.k8sNameSpace), slog.String("pod-name", podName),
			slog.Any("svcname", svcName), slog.Any("error", err))
		logs = "no any logs found for the service"
	}

	setResponseHeaderForLogs(c)

	_, err = c.Writer.Write([]byte(logs))
	if err != nil {
		slog.Error("write pod log msg failed",
			slog.Any("namespace", s.k8sNameSpace), slog.Any("pod-name", podName),
			slog.Any("clusterID", cluster.ID), slog.Any("error", err))
	}
	c.Writer.Flush()
}

func (s *K8sHandler) readPodLogsFromCluster(c *gin.Context, cluster cluster.Cluster, podName, svcName string) {
	slog.Debug("read pod logs from cluster", slog.Any("namespace", s.k8sNameSpace), slog.Any("pod-name", podName),
		slog.Any("svcname", svcName), slog.Any("clusterID", cluster.ID))
	ch, message, err := rcommon.GetPodLogStream(c, &cluster, podName, s.k8sNameSpace, rTypes.UserContainerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open stream"})
		return
	}
	defer func() {
		if ch != nil {
			select {
			case _, ok := <-ch:
				if ok {
					close(ch)
				}
			default:
				close(ch)
			}
		}
	}()

	setResponseHeaderForLogs(c)

	if message != "" {
		_, err = c.Writer.Write([]byte(message))
		if err != nil {
			slog.Error("write pod message data failed", slog.Any("svcName", svcName),
				slog.Any("namespace", s.k8sNameSpace), slog.Any("pod-name", podName),
				slog.Any("clusterID", cluster.ID), slog.Any("error", err))
		}
		c.Writer.Flush()
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
		return
	}

	for log := range ch {
		_, err := c.Writer.Write(log)
		if err != nil {
			slog.Error("write pod log data failed", slog.Any("svcName", svcName),
				slog.Any("namespace", s.k8sNameSpace), slog.Any("pod-name", podName),
				slog.Any("clusterID", cluster.ID), slog.Any("error", err))
		}
		c.Writer.Flush()
	}
}

func setResponseHeaderForLogs(c *gin.Context) {
	c.Header("Content-Type", "text/plain")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Writer.WriteHeader(http.StatusOK)
}

func (s *K8sHandler) ServiceStatusAll(c *gin.Context) {
	allStatus, err := s.serviceComponent.GetAllServiceStatus(c)
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
		if !cInfo.Enable {
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
	svcName := s.getServiceNameFromRequest(c)
	svc, err := s.serviceComponent.GetServiceByName(c.Request.Context(), svcName, request.ClusterID)
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
	resp.Message = svcName
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
	svcName := s.getServiceNameFromRequest(c)
	svc, err := s.serviceComponent.GetServiceByName(c.Request.Context(), svcName, request.ClusterID)
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
	resp.Message = svcName
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
	svcName := s.getServiceNameFromRequest(c)
	request.SvcName = svcName
	resp, err = s.serviceComponent.PurgeService(c.Request.Context(), *request)
	if err != nil {
		slog.Error("fail to purge service", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	slog.Info("service deleted by purge service", slog.String("svc_name", svcName))
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

	svcName := s.getServiceNameFromRequest(c)
	request.ServiceName = svcName
	resp, err := s.serviceComponent.GetServiceInfo(c.Request.Context(), *request)
	if err != nil {
		slog.Error("fail to get service info", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fail to get service info"})
		return
	}
	c.JSON(http.StatusOK, resp)
}
